//go:build integration

// Integration tests build the pluck binary and run it against the fixture
// vault in testdata/vault. Run them with `make test-integration`.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	binary   string
	testdata string
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	dir, err := os.MkdirTemp("", "pluck-integration-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	binary = filepath.Join(dir, "pluck")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build pluck:", err)
		return 1
	}

	if testdata, err = filepath.Abs("testdata"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return m.Run()
}

// allFiles is every non-hidden file in the fixture vault, in output order.
var allFiles = []string{
	"Blog/2026/draft.md",
	"Blog/2026/first-post.md",
	"Blog/images/cover.png",
	"Canvas/board.canvas",
	"Daily/2026-10-17.md",
	"Daily/2026-10-18.md",
	"Home.md",
	"Inbox/broken.md",
	"Projects/core/notes",
	"Projects/core/spec.md",
	"Projects/other/readme.md",
}

type result struct {
	stdout, stderr string
	code           int
}

// pluck runs the binary from the testdata directory, so "vault" and
// "configs/..." resolve to the fixtures.
func pluck(t *testing.T, args ...string) result {
	t.Helper()
	return pluckIn(t, testdata, nil, args...)
}

func pluckIn(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	c := exec.Command(binary, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	c.Stdout, c.Stderr = &stdout, &stderr

	err := c.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else {
		require.NoError(t, err, "pluck failed to start")
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func lines(paths ...string) string {
	if len(paths) == 0 {
		return ""
	}
	return strings.Join(paths, "\n") + "\n"
}

// treeOf lists every file under root as slash-separated relative paths.
func treeOf(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		files = append(files, filepath.ToSlash(rel))
		return err
	})
	require.NoError(t, err)
	return files
}

func assertSameContent(t *testing.T, target string, paths []string) {
	t.Helper()
	for _, p := range paths {
		want, err := os.ReadFile(filepath.Join(testdata, "vault", filepath.FromSlash(p)))
		require.NoError(t, err)
		got, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(p)))
		require.NoError(t, err)
		assert.Equal(t, want, got, "content of %s", p)
	}
}

// --- Happy paths -----------------------------------------------------------

func TestDryRunFilters(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"directory with **", []string{"--dir", "Blog/**"},
			[]string{"Blog/2026/draft.md", "Blog/2026/first-post.md", "Blog/images/cover.png"}},
		{"directory at any depth", []string{"--dir", "**/core"},
			[]string{"Projects/core/notes", "Projects/core/spec.md"}},
		{"directory alternatives", []string{"--dir", "{Daily,Canvas}"},
			[]string{"Canvas/board.canvas", "Daily/2026-10-17.md", "Daily/2026-10-18.md"}},
		{"empty directory pattern is the vault root", []string{"--dir", ""},
			[]string{"Home.md"}},
		{"name by extension", []string{"--name", "*.md"},
			[]string{"Blog/2026/draft.md", "Blog/2026/first-post.md", "Daily/2026-10-17.md", "Daily/2026-10-18.md",
				"Home.md", "Inbox/broken.md", "Projects/core/spec.md", "Projects/other/readme.md"}},
		{"name with character classes", []string{"--name", "[0-9][0-9][0-9][0-9]-*.md"},
			[]string{"Daily/2026-10-17.md", "Daily/2026-10-18.md"}},
		{"name matches any file type", []string{"--name", "*.{png,canvas}"},
			[]string{"Blog/images/cover.png", "Canvas/board.canvas"}},
		{"name without extension", []string{"--name", "notes"},
			[]string{"Projects/core/notes"}},
		{"frontmatter scalar", []string{"--frontmatter", "status=published"},
			[]string{"Blog/2026/first-post.md"}},
		{"frontmatter list and scalar tags", []string{"--frontmatter", "tags=share"},
			[]string{"Blog/2026/first-post.md", "Daily/2026-10-18.md"}},
		{"frontmatter date", []string{"--frontmatter", "date=2026-10-*"},
			[]string{"Blog/2026/first-post.md"}},
		{"frontmatter field present", []string{"--frontmatter", "title=*"},
			[]string{"Blog/2026/draft.md", "Blog/2026/first-post.md", "Home.md"}},
		{"filters are ORed", []string{"--dir", "**/core", "--frontmatter", "status=draft", "--name", "*.canvas"},
			[]string{"Blog/2026/draft.md", "Canvas/board.canvas", "Projects/core/notes", "Projects/core/spec.md"}},
		{"hidden files and folders are never matched", []string{"--name", "*"}, allFiles},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := pluck(t, append([]string{"--source", "vault"}, tt.args...)...)

			assert.Equal(t, 0, res.code, res.stderr)
			assert.Equal(t, lines(tt.want...), res.stdout)
			assert.Contains(t, res.stderr, fmt.Sprintf("%d file(s) matched (dry run", len(tt.want)))
		})
	}
}

func TestDryRunWithNoMatches(t *testing.T) {
	res := pluck(t, "--source", "vault", "--name", "*.pdf")

	assert.Equal(t, 0, res.code)
	assert.Empty(t, res.stdout)
	assert.Contains(t, res.stderr, "0 file(s) matched")
}

func TestDryRunNeverWrites(t *testing.T) {
	target := filepath.Join(t.TempDir(), "out")

	res := pluck(t, "--source", "vault", "--target", target, "--name", "*")

	assert.Equal(t, 0, res.code)
	assert.Equal(t, lines(allFiles...), res.stdout)
	assert.NoDirExists(t, target)
}

func TestBrokenFrontmatterWarnsButSucceeds(t *testing.T) {
	res := pluck(t, "--source", "vault", "--name", "broken.md")

	assert.Equal(t, 0, res.code)
	assert.Equal(t, "Inbox/broken.md\n", res.stdout, "the file is still found by name")
	assert.Contains(t, res.stderr, "warning: Inbox/broken.md: invalid frontmatter")
}

func TestConfigFile(t *testing.T) {
	res := pluck(t, "--config", "configs/blog.yaml")

	assert.Equal(t, 0, res.code, res.stderr)
	assert.Equal(t, lines("Blog/2026/draft.md", "Blog/2026/first-post.md", "Blog/images/cover.png", "Daily/2026-10-18.md"), res.stdout)
}

func TestFlagsOverrideConfigFile(t *testing.T) {
	// --dir replaces the config's directories; its frontmatter rule still applies.
	res := pluck(t, "-c", "configs/blog.yaml", "--dir", "Projects/**")

	assert.Equal(t, 0, res.code, res.stderr)
	assert.Equal(t, lines("Blog/2026/first-post.md", "Daily/2026-10-18.md",
		"Projects/core/notes", "Projects/core/spec.md", "Projects/other/readme.md"), res.stdout)
}

func TestHomeDirectoryExpansion(t *testing.T) {
	res := pluckIn(t, t.TempDir(), []string{"HOME=" + testdata}, "--source", "~/vault", "--name", "Home.md")

	assert.Equal(t, 0, res.code, res.stderr)
	assert.Equal(t, "Home.md\n", res.stdout)
}

func TestHelp(t *testing.T) {
	res := pluck(t, "--help")

	assert.Equal(t, 0, res.code)
	assert.Contains(t, res.stdout, "Usage:")
	assert.Contains(t, res.stdout, "--copy")
}

func TestCopyFromConfig(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "out")
	want := []string{"Blog/2026/draft.md", "Blog/2026/first-post.md", "Blog/images/cover.png", "Daily/2026-10-18.md"}

	res := pluck(t, "-c", "configs/blog.yaml", "--target", target, "--copy")

	assert.Equal(t, 0, res.code, res.stderr)
	assert.Equal(t, lines(want...), res.stdout)
	assert.Contains(t, res.stderr, "copied 4 file(s) to "+target)
	assert.Equal(t, want, treeOf(t, target), "folder structure is kept")
	assertSameContent(t, target, want)
}

func TestCopyEverythingSkipsHiddenFiles(t *testing.T) {
	target := t.TempDir()

	res := pluck(t, "--source", "vault", "--target", target, "--name", "*", "--copy")

	assert.Equal(t, 0, res.code, res.stderr)
	assert.Equal(t, allFiles, treeOf(t, target))
	assertSameContent(t, target, allFiles)
}

func TestCopyOverwritesButKeepsOtherFiles(t *testing.T) {
	target := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(target, "Home.md"), []byte("stale"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(target, "unrelated.txt"), []byte("keep me"), 0o644))

	for range 2 { // running twice gives the same result
		res := pluck(t, "--source", "vault", "--target", target, "--name", "Home.md", "--copy")
		require.Equal(t, 0, res.code, res.stderr)
	}

	assertSameContent(t, target, []string{"Home.md"})
	assert.Equal(t, []string{"Home.md", "unrelated.txt"}, treeOf(t, target))
}

// --- Unhappy paths ---------------------------------------------------------

func TestErrors(t *testing.T) {
	aFile := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(aFile, nil, 0o644))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no arguments", nil, "a source vault is required"},
		{"no filters", []string{"--source", "vault"}, "at least one filter is required"},
		{"source does not exist", []string{"--source", "missing", "--dir", "x"}, "source missing does not exist"},
		{"source is a file", []string{"--source", "vault/Home.md", "--dir", "x"}, "is not a directory"},
		{"invalid directory glob", []string{"--source", "vault", "--dir", "[unclosed"}, `invalid directory pattern "[unclosed"`},
		{"invalid name glob", []string{"--source", "vault", "--name", "{a,b"}, `invalid name pattern "{a,b"`},
		{"invalid frontmatter glob", []string{"--source", "vault", "--frontmatter", "status=[x"}, `invalid frontmatter pattern "[x" for "status"`},
		{"frontmatter flag without =", []string{"--source", "vault", "--frontmatter", "status"}, "must be in the form field=pattern"},
		{"frontmatter flag without field", []string{"--source", "vault", "--frontmatter", "=draft"}, "must be in the form field=pattern"},
		{"config file missing", []string{"-c", "configs/missing.yaml"}, "read config"},
		{"config file invalid yaml", []string{"-c", "configs/invalid-yaml.yaml"}, "read config"},
		{"config with bad pattern", []string{"-c", "configs/bad-pattern.yaml"}, `invalid name pattern "[unclosed"`},
		{"config frontmatter rule without field", []string{"-c", "configs/missing-field.yaml"}, "frontmatter filter is missing a field"},
		{"config without filters", []string{"-c", "configs/no-filters.yaml"}, "at least one filter is required"},
		{"copy without target", []string{"--source", "vault", "--dir", "x", "--copy"}, "a target directory is required"},
		{"target inside source", []string{"--source", "vault", "--dir", "x", "--target", "vault/out"}, "is inside the source vault"},
		{"target is the source", []string{"--source", "vault", "--dir", "x", "--target", "vault"}, "is inside the source vault"},
		{"target is a file", []string{"--source", "vault", "--name", "Home.md", "--target", aFile, "--copy"}, "not a directory"},
		{"unknown flag", []string{"--bogus"}, "unknown flag: --bogus"},
		{"positional argument", []string{"vault"}, `unknown command "vault"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := pluck(t, tt.args...)

			assert.Equal(t, 1, res.code)
			assert.Empty(t, res.stdout, "errors must not print file paths")
			assert.Contains(t, res.stderr, "Error: ")
			assert.Contains(t, res.stderr, tt.want)
		})
	}
}

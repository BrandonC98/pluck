package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// newVault builds a small vault used by the command tests.
func newVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "Blog/post.md", "---\nstatus: draft\n---\n")
	writeFile(t, root, "Notes/published.md", "---\nstatus: published\n---\n")
	writeFile(t, root, "Notes/tagged.md", "---\ntags: [work, share]\n---\n")
	writeFile(t, root, "Notes/2026-10-17.md", "daily")
	writeFile(t, root, "Notes/other.md", "nothing to see")
	writeFile(t, root, "core/myfile", "no extension")
	writeFile(t, root, "core/diagram.png", "image")
	return root
}

func execute(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := NewRootCmd()
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetArgs(args)
	err = c.Execute()
	return out.String(), errOut.String(), err
}

func TestDryRunIsDefault(t *testing.T) {
	vault := newVault(t)
	target := filepath.Join(t.TempDir(), "out")

	stdout, _, err := execute(t, "--source", vault, "--target", target, "--dir", "Blog")
	require.NoError(t, err)

	assert.Equal(t, "Blog/post.md\n", stdout)
	assert.NoDirExists(t, target, "dry run must not write anything")
}

func TestFiltersAreORed(t *testing.T) {
	vault := newVault(t)

	stdout, _, err := execute(t, "--source", vault,
		"--dir", "Blog",
		"--name", "[0-9][0-9][0-9][0-9]-*.md",
		"--frontmatter", "status=published",
		"--frontmatter", "tags=share",
	)
	require.NoError(t, err)

	assert.Equal(t, "Blog/post.md\nNotes/2026-10-17.md\nNotes/published.md\nNotes/tagged.md\n", stdout)
}

func TestCopyKeepsStructure(t *testing.T) {
	vault := newVault(t)
	target := filepath.Join(t.TempDir(), "out")

	_, _, err := execute(t, "--source", vault, "--target", target, "--frontmatter", "status=*", "--copy")
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(target, "Blog", "post.md"))
	assert.FileExists(t, filepath.Join(target, "Notes", "published.md"))
	assert.NoFileExists(t, filepath.Join(target, "Notes", "other.md"))
}

func TestConfigFile(t *testing.T) {
	vault := newVault(t)
	target := filepath.Join(t.TempDir(), "out")
	cfgDir := t.TempDir()
	writeFile(t, cfgDir, "sync.yaml", `
source: `+vault+`
target: `+target+`
filters:
  directories: ["Blog"]
  frontmatter:
    - field: status
      pattern: "published"
`)

	stdout, _, err := execute(t, "--config", filepath.Join(cfgDir, "sync.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "Blog/post.md\nNotes/published.md\n", stdout)

	// Flags override values from the config file.
	stdout, _, err = execute(t, "--config", filepath.Join(cfgDir, "sync.yaml"), "--dir", "Nowhere", "--frontmatter", "tags=work")
	require.NoError(t, err)
	assert.Equal(t, "Notes/tagged.md\n", stdout)
}

func TestValidation(t *testing.T) {
	vault := newVault(t)
	aFile := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(aFile, nil, 0o644))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing source", []string{"--dir", "x"}, "source"},
		{"no filters", []string{"--source", vault}, "at least one filter"},
		{"bad glob", []string{"--source", vault, "--dir", "[unclosed"}, "invalid directory pattern"},
		{"bad frontmatter flag", []string{"--source", vault, "--frontmatter", "status"}, "field=pattern"},
		{"copy without target", []string{"--source", vault, "--dir", "x", "--copy"}, "target"},
		{"target inside source", []string{"--source", vault, "--dir", "x", "--target", filepath.Join(vault, "out")}, "inside the source"},
		{"source does not exist", []string{"--source", filepath.Join(vault, "missing"), "--dir", "x"}, "does not exist"},
		{"source is a file", []string{"--source", filepath.Join(vault, "Notes", "other.md"), "--dir", "x"}, "source " + filepath.Join(vault, "Notes", "other.md") + " is not a directory"},
		{"target is a file", []string{"--source", vault, "--dir", "x", "--target", aFile}, "target " + aFile + " is not a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := execute(t, tt.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestEmptyDirPatternMatchesVaultRoot(t *testing.T) {
	vault := newVault(t)
	writeFile(t, vault, "root.md", "at the root")

	stdout, _, err := execute(t, "--source", vault, "--dir", "")
	require.NoError(t, err)
	assert.Equal(t, "root.md\n", stdout)
}

func TestMatchesAnyFileType(t *testing.T) {
	vault := newVault(t)

	stdout, _, err := execute(t, "--source", vault, "--dir", "**/core/**")
	require.NoError(t, err)
	assert.Equal(t, "core/diagram.png\ncore/myfile\n", stdout)
}

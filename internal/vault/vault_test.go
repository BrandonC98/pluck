package vault

import (
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

func TestScan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "root.md", "---\nstatus: published\n---\n")
	writeFile(t, root, "Blog/post.MD", "no frontmatter")
	writeFile(t, root, "Blog/image.png", "binary")
	writeFile(t, root, ".obsidian/workspace.md", "ignored")
	writeFile(t, root, "Blog/.hidden/note.md", "ignored")
	writeFile(t, root, "broken.md", "---\nstatus: [unclosed\n---\n")
	writeFile(t, root, "core/myfile", "no extension")
	writeFile(t, root, "Blog/.DS_Store", "ignored")
	writeFile(t, root, "data.txt", "---\nstatus: published\n---\n")

	files, warnings, err := Scan(root)
	require.NoError(t, err)

	byPath := map[string]map[string]any{}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.RelPath)
		byPath[f.RelPath] = f.Frontmatter
	}
	assert.Equal(t, []string{"Blog/image.png", "Blog/post.MD", "broken.md", "core/myfile", "data.txt", "root.md"}, paths,
		"every file type is found, hidden files and folders are skipped")
	assert.Equal(t, map[string]any{"status": "published"}, byPath["root.md"])
	assert.Nil(t, byPath["data.txt"], "frontmatter is only read from markdown files")
	assert.Nil(t, byPath["Blog/image.png"])

	require.Len(t, warnings, 1, "broken frontmatter is reported, not fatal")
	assert.Contains(t, warnings[0].Error(), "broken.md")
}

func TestScanMissingRoot(t *testing.T) {
	_, _, err := Scan(filepath.Join(t.TempDir(), "missing"))
	assert.Error(t, err)
}

func TestCopy(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")
	writeFile(t, src, "Blog/2026/post.md", "post")
	writeFile(t, src, "root.md", "root")
	writeFile(t, dst, "root.md", "stale")

	require.NoError(t, Copy(src, dst, []string{"Blog/2026/post.md", "root.md"}))

	got, err := os.ReadFile(filepath.Join(dst, "Blog", "2026", "post.md"))
	require.NoError(t, err)
	assert.Equal(t, "post", string(got), "folder structure is preserved")

	got, err = os.ReadFile(filepath.Join(dst, "root.md"))
	require.NoError(t, err)
	assert.Equal(t, "root", string(got), "existing files are overwritten")
}

package filter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDirectory(t *testing.T) {
	f := Directory{Pattern: "Blog/**"}

	assert.True(t, f.Match(File{RelPath: "Blog/post.md"}))
	assert.True(t, f.Match(File{RelPath: "Blog/2026/post.md"}))
	assert.False(t, f.Match(File{RelPath: "Blogging/post.md"}))
	assert.False(t, f.Match(File{RelPath: "Journal/Blog/post.md"}))
}

func TestDirectoryWildcards(t *testing.T) {
	assert.True(t, Directory{Pattern: "**/Blog"}.Match(File{RelPath: "Journal/Blog/post.md"}))
	assert.True(t, Directory{Pattern: "{Blog,Notes}"}.Match(File{RelPath: "Notes/a.md"}))
	// A single * stays within one folder level.
	assert.True(t, Directory{Pattern: "Blog/*"}.Match(File{RelPath: "Blog/2026/post.md"}))
	assert.False(t, Directory{Pattern: "Blog/*"}.Match(File{RelPath: "Blog/2026/10/post.md"}))
}

func TestDirectoryRootIsEmptyString(t *testing.T) {
	f := Directory{Pattern: ""}

	assert.True(t, f.Match(File{RelPath: "root.md"}))
	assert.False(t, f.Match(File{RelPath: "sub/nested.md"}))
}

func TestName(t *testing.T) {
	f := Name{Pattern: "[0-9][0-9][0-9][0-9]-*.md"}

	assert.True(t, f.Match(File{RelPath: "Daily/2026-10-17.md"}))
	assert.False(t, f.Match(File{RelPath: "Daily/notes.md"}))
	// Only the base name is matched, never the directory.
	assert.False(t, Name{Pattern: "Daily*"}.Match(File{RelPath: "Daily/notes.md"}))
}

func TestFrontmatter(t *testing.T) {
	status := Frontmatter{Field: "status", Pattern: "published"}
	tags := Frontmatter{Field: "tags", Pattern: "blog"}
	prefix := Frontmatter{Field: "tags", Pattern: "proj-*"}
	date := Frontmatter{Field: "date", Pattern: "2026-10-*"}
	count := Frontmatter{Field: "count", Pattern: "3"}

	tests := []struct {
		name   string
		filter Frontmatter
		fm     map[string]any
		want   bool
	}{
		{"scalar match", status, map[string]any{"status": "published"}, true},
		{"glob matches whole value", status, map[string]any{"status": "unpublished"}, false},
		{"scalar mismatch", status, map[string]any{"status": "draft"}, false},
		{"missing field", status, map[string]any{"title": "x"}, false},
		{"nil frontmatter", status, nil, false},
		{"list contains match", tags, map[string]any{"tags": []any{"work", "blog"}}, true},
		{"list without match", tags, map[string]any{"tags": []any{"work", "blogging"}}, false},
		{"list wildcard", prefix, map[string]any{"tags": []any{"work", "proj-x"}}, true},
		{"scalar tag", tags, map[string]any{"tags": "blog"}, true},
		{"non-string scalar", count, map[string]any{"count": 3}, true},
		{"date value", date, map[string]any{"date": time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)}, true},
		{"nested map never matches", status, map[string]any{"status": map[string]any{"a": "published"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.filter.Match(File{RelPath: "a.md", Frontmatter: tt.fm}))
		})
	}
}

func TestValidate(t *testing.T) {
	assert.NoError(t, Validate("Blog/**"))
	assert.Error(t, Validate("Blog/[unclosed"))
}

func TestAny(t *testing.T) {
	dir := Directory{Pattern: "Blog"}
	name := Name{Pattern: "readme.md"}
	f := Any{dir, name}

	assert.True(t, f.Match(File{RelPath: "Blog/post.md"}))
	assert.True(t, f.Match(File{RelPath: "Other/readme.md"}))
	assert.False(t, f.Match(File{RelPath: "Other/post.md"}))
	assert.False(t, Any{}.Match(File{RelPath: "Blog/post.md"}), "no filters matches nothing")
}

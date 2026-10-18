// Package filter decides which markdown files in a vault are selected.
package filter

import (
	"fmt"
	"path"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// File is a markdown file found in the vault.
type File struct {
	// RelPath is the path relative to the vault root, using forward slashes.
	RelPath     string
	Frontmatter map[string]any
}

// Validate reports whether pattern is a valid glob. Patterns support *, ?,
// [abc], {a,b} and ** (any number of folders).
func Validate(pattern string) error {
	if !doublestar.ValidatePattern(pattern) {
		return doublestar.ErrBadPattern
	}
	return nil
}

// match reports whether the whole of value matches the glob pattern.
// Invalid patterns never match; check them with Validate first.
func match(pattern, value string) bool {
	ok, _ := doublestar.Match(pattern, value)
	return ok
}

// Filter reports whether a file should be selected.
type Filter interface {
	Match(f File) bool
}

// Any selects a file when at least one of its filters matches (logical OR).
// An AND combinator can be added alongside it later without changing the filters.
type Any []Filter

func (a Any) Match(f File) bool {
	for _, filter := range a {
		if filter.Match(f) {
			return true
		}
	}
	return false
}

// Directory matches the file's directory relative to the vault root,
// e.g. "Blog/2026". Files at the root have the directory "".
type Directory struct {
	Pattern string
}

func (d Directory) Match(f File) bool {
	dir := path.Dir(f.RelPath)
	if dir == "." {
		dir = ""
	}
	return match(d.Pattern, dir)
}

// Name matches the file's base name, including its extension.
type Name struct {
	Pattern string
}

func (n Name) Match(f File) bool {
	return match(n.Pattern, path.Base(f.RelPath))
}

// Frontmatter matches a frontmatter field's value. For list fields such as
// tags, the file matches when any element matches.
type Frontmatter struct {
	Field   string
	Pattern string
}

func (fm Frontmatter) Match(f File) bool {
	value, ok := f.Frontmatter[fm.Field]
	if !ok {
		return false
	}
	if list, ok := value.([]any); ok {
		for _, item := range list {
			if fm.matchScalar(item) {
				return true
			}
		}
		return false
	}
	return fm.matchScalar(value)
}

func (fm Frontmatter) matchScalar(value any) bool {
	switch v := value.(type) {
	case nil, map[string]any, []any:
		return false
	case time.Time:
		return match(fm.Pattern, formatTime(v))
	default:
		return match(fm.Pattern, fmt.Sprint(v))
	}
}

// formatTime renders dates the way they are usually written in frontmatter.
func formatTime(t time.Time) string {
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return t.Format(time.DateOnly)
	}
	return t.Format(time.RFC3339)
}

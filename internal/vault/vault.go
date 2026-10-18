// Package vault finds files in an Obsidian vault and copies them out.
package vault

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brandonc98/pluck/internal/filter"
	"github.com/brandonc98/pluck/internal/frontmatter"
)

// Scan walks root and returns every file, sorted by path. Hidden files and
// directories such as .obsidian, .trash and .DS_Store are skipped.
// Frontmatter is only read from markdown files. Markdown files whose
// frontmatter cannot be parsed are still returned, with no frontmatter, and
// reported as warnings.
func Scan(root string) (files []filter.File, warnings []error, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		hidden := path != root && strings.HasPrefix(d.Name(), ".")
		if d.IsDir() {
			if hidden {
				return filepath.SkipDir
			}
			return nil
		}
		if hidden || !d.Type().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		file := filter.File{RelPath: rel}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			file.Frontmatter, err = readFrontmatter(path)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("%s: %w", rel, err))
			}
		}
		files = append(files, file)
		return nil
	})
	return files, warnings, err
}

func readFrontmatter(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return frontmatter.Parse(f)
}

// Copy copies each file in relPaths from src to dst, recreating the
// directory structure and overwriting files that already exist.
func Copy(src, dst string, relPaths []string) error {
	for _, rel := range relPaths {
		from := filepath.Join(src, filepath.FromSlash(rel))
		to := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return err
		}
		if err := copyFile(from, to); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

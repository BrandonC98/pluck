// Package frontmatter extracts the YAML frontmatter block from a markdown file.
package frontmatter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

const delimiter = "---"

// Parse reads the YAML block delimited by "---" lines at the very start of r.
// A file without frontmatter yields an empty map and no error.
func Parse(r io.Reader) (map[string]any, error) {
	scanner := bufio.NewScanner(r)
	fields := map[string]any{}

	if !scanner.Scan() || trimLine(scanner.Text()) != delimiter {
		return fields, scanner.Err()
	}

	var block strings.Builder
	closed := false
	for scanner.Scan() {
		line := trimLine(scanner.Text())
		if line == delimiter || line == "..." {
			closed = true
			break
		}
		block.WriteString(line)
		block.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !closed {
		return nil, errors.New("frontmatter is not closed with ---")
	}

	if err := yaml.Unmarshal([]byte(block.String()), &fields); err != nil {
		return nil, fmt.Errorf("invalid frontmatter: %w", err)
	}
	if fields == nil {
		fields = map[string]any{}
	}
	return fields, nil
}

func trimLine(s string) string {
	return strings.TrimRight(s, "\r \t")
}

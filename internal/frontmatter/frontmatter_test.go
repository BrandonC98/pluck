package frontmatter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]any
		wantErr bool
	}{
		{
			name:  "scalar and list fields",
			input: "---\ntitle: Hello\ntags: [blog, work]\n---\n# Body\n",
			want:  map[string]any{"title": "Hello", "tags": []any{"blog", "work"}},
		},
		{
			name:  "no frontmatter",
			input: "# Just a heading\n",
			want:  map[string]any{},
		},
		{
			name:  "empty frontmatter",
			input: "---\n---\nbody\n",
			want:  map[string]any{},
		},
		{
			name:  "windows line endings",
			input: "---\r\nstatus: draft\r\n---\r\nbody\r\n",
			want:  map[string]any{"status": "draft"},
		},
		{
			name:  "horizontal rule later in file is not frontmatter",
			input: "intro\n---\nstatus: draft\n---\n",
			want:  map[string]any{},
		},
		{
			name:    "unterminated frontmatter",
			input:   "---\nstatus: draft\n",
			wantErr: true,
		},
		{
			name:    "invalid yaml",
			input:   "---\nstatus: [unclosed\n---\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(strings.NewReader(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

# pluck

Pluck matching files out of a directory tree, such as an Obsidian vault, and
copy them somewhere else.

Files are selected by **folder**, **file name** or **markdown frontmatter
field**, using glob patterns. By default pluck only lists what matches (a dry
run); pass `--copy` to copy the files to a target directory, keeping their
folder structure.

```sh
# What would be exported?
pluck --source ~/vault --dir 'Blog/**' --frontmatter 'tags=share'

# Export it
pluck --source ~/vault --dir 'Blog/**' --frontmatter 'tags=share' --target ./export --copy

# Or keep the search in a config file
pluck --config blog.yaml --copy
```

## Contents

- [Install](#install)
- [How matching works](#how-matching-works)
- [Command-line usage](#command-line-usage)
- [Config file](#config-file)
- [Copying](#copying)
- [Output and exit codes](#output-and-exit-codes)
- [Development](#development)

## Install

Requires Go 1.27 or later. Install the latest version onto your `PATH`
(into `$(go env GOPATH)/bin`):

```sh
go install github.com/brandonc98/pluck@latest
```

Or build it from a clone. Go can be installed with Homebrew from the included
`Brewfile`:

```sh
git clone https://github.com/brandonc98/pluck.git
cd pluck
make deps    # brew bundle: installs Go
make build   # builds bin/pluck
```

## How matching works

pluck walks the source directory and checks every file against the filters.
**A file is selected when any filter matches** (filters are combined with OR).
At least one filter is required.

| Filter | Flag | Matched against | Example value |
|---|---|---|---|
| Directory | `--dir` | The file's folder, relative to the source | `Blog/2026` |
| File name | `--name` | The file's name, including its extension | `2026-10-17.md` |
| Frontmatter | `--frontmatter field=pattern` | A field in a markdown file's YAML frontmatter | `status: published` |

### Glob patterns

All patterns are globs and must match the **whole** value, so `published`
does not match `unpublished` (use `*published*` for that).

| Syntax | Matches | Example |
|---|---|---|
| `*` | Anything except `/` | `*.md`, `Blog/*` |
| `**` | Any number of folders, including none | `Blog/**`, `**/core` |
| `?` | Exactly one character | `note-?.md` |
| `[abc]`, `[0-9]` | One character from the set or range | `[0-9][0-9][0-9][0-9]-*.md` |
| `{a,b}` | Either alternative | `*.{png,jpg}`, `{Daily,Weekly}` |

### Directory patterns

The folder is relative to the source and uses `/` on every platform.

| Pattern | Selects files in |
|---|---|
| `Blog` | `Blog` only, not its subfolders |
| `Blog/*` | Folders directly inside `Blog` |
| `Blog/**` | `Blog` and every folder below it |
| `**/core` | Any folder named `core`, at any depth |
| `''` (empty) | The root of the source only |

### File name patterns

Only the file name is matched, never its folder. Any file type can be
matched, including files without an extension.

```sh
pluck -s ~/vault --name '*.md'                       # every markdown note
pluck -s ~/vault --name '*.{png,jpg,gif}'            # images
pluck -s ~/vault --name '[0-9][0-9][0-9][0-9]-*.md'  # daily notes
```

### Frontmatter patterns

Frontmatter is the YAML block between `---` lines at the very top of a
markdown file:

```markdown
---
status: published
date: 2026-10-01
tags: [blog, share]
---
# My post
```

- **List fields** such as `tags` match when **any** item matches:
  `tags=share` selects the note above.
- **Dates** are matched in `YYYY-MM-DD` form: `date=2026-10-*`.
- **Numbers and booleans** are matched as text: `draft=true`, `rating=5`.
- **Field names are case-sensitive**: `Status` and `status` are different
  fields.
- `field=*` selects every note that has the field, whatever its value.
- Nested objects never match.
- Frontmatter is only read from `.md` files, so frontmatter filters never
  select other file types.
- A note whose frontmatter is invalid YAML is reported as a warning and
  treated as having no frontmatter. It can still be selected by folder or name.

### What is always skipped

- Hidden files and folders, i.e. anything whose name starts with `.`, such as
  `.obsidian/`, `.trash/`, `.git/` and `.DS_Store`.
- Symlinks and other special files.

## Command-line usage

```
pluck [flags]
```

| Flag | Description |
|---|---|
| `-c, --config <file>` | YAML config file with the search parameters |
| `-s, --source <dir>` | Directory to search (required) |
| `-t, --target <dir>` | Directory to copy matching files into (required with `--copy`) |
| `--copy` | Copy the matching files. Without it, pluck only lists them |
| `--dir <glob>` | Directory filter. Repeatable |
| `--name <glob>` | File name filter. Repeatable |
| `--frontmatter <field=glob>` | Frontmatter filter. Repeatable |
| `-h, --help` | Show help |

Quote patterns so your shell doesn't expand them: `--name '*.md'`, not
`--name *.md`.

Paths starting with `~/` are expanded to your home directory.

### Examples

```sh
# Notes tagged "share" or with status "published"
pluck -s ~/vault --frontmatter 'tags=share' --frontmatter 'status=published'

# Everything in any "core" folder, plus all canvases
pluck -s ~/vault --dir '**/core' --name '*.canvas'

# Copy the blog folder, attachments included
pluck -s ~/vault -t ~/site/content --dir 'Blog/**' --copy

# Count matches (file paths go to stdout, the summary to stderr)
pluck -s ~/vault --name '*.md' | wc -l
```

## Config file

A config file holds the same settings as the flags, so a search can be saved
and rerun:

```sh
pluck --config blog.yaml          # dry run
pluck --config blog.yaml --copy   # copy
```

### Format

```yaml
# Directory to search.
source: ~/Obsidian/MyVault

# Where --copy puts files. Not needed for a dry run.
target: ./export

# Set to true to copy without passing --copy. Defaults to false (dry run).
copy: false

# A file is selected when ANY of these patterns match.
filters:
  # Globs matched against the file's folder, relative to source.
  directories:
    - "Blog/**"
    - "**/core"

  # Globs matched against the file name.
  filenames:
    - "[0-9][0-9][0-9][0-9]-*.md"
    - "*.canvas"

  # Globs matched against a markdown frontmatter field.
  # List fields (like tags) match when any item matches.
  frontmatter:
    - field: status
      pattern: "published"
    - field: tags
      pattern: "{share,public}"
```

### Keys

| Key | Type | Description |
|---|---|---|
| `source` | string | Directory to search. Required |
| `target` | string | Directory to copy into. Required when copying |
| `copy` | bool | Copy instead of a dry run. Default `false` |
| `filters.directories` | list of globs | Directory filters |
| `filters.filenames` | list of globs | File name filters |
| `filters.frontmatter` | list of `{field, pattern}` | Frontmatter filters. Both keys are required |

All filter lists are optional, but at least one filter must be set.

### Tips

- **Quote patterns.** In YAML, a value starting with `*`, `[`, `{` or `?` has
  a special meaning, so `"*.md"` must be quoted. Quoting every pattern is
  simplest.
- **Relative paths** for `source` and `target` are resolved from the folder
  you run pluck in, not from the config file's folder. Use absolute paths or
  `~/` to make a config work from anywhere.
- **Flags override the config.** A flag that is set replaces the matching
  config value. For filters, the whole list is replaced: `--dir 'Notes/**'`
  replaces `filters.directories`, while `filenames` and `frontmatter` from the
  config still apply.

```sh
# Same config, different target and directories
pluck -c blog.yaml -t /tmp/preview --dir 'Drafts/**'
```

## Copying

With `--copy`, each matching file is copied to the same relative path under
the target:

```
~/vault/Blog/2026/post.md      ->  ./export/Blog/2026/post.md
~/vault/Blog/images/cover.png  ->  ./export/Blog/images/cover.png
```

- Missing folders in the target are created.
- Existing files in the target are **overwritten**.
- Files in the target that don't match are **left alone**. pluck never
  deletes anything, so rerunning the same command is safe.
- The target must not be inside the source, because the copies would be found
  again on the next run.
- The target is checked before anything is scanned; an invalid target (such
  as an existing file) fails without copying anything.

## Output and exit codes

- **stdout:** one matching path per line, relative to the source, sorted.
  Ready to pipe into other tools.
- **stderr:** the summary (`3 file(s) matched (dry run, pass --copy to copy
  them)` or `copied 3 file(s) to ./export`), frontmatter warnings, and errors.

| Exit code | Meaning |
|---|---|
| `0` | Success, including when nothing matched |
| `1` | Error: invalid flags or config, a bad pattern, a missing source, or a failed copy. Nothing is printed to stdout |

## Development

| Command | What it does |
|---|---|
| `make deps` | Install tooling from the `Brewfile` |
| `make build` | Build `bin/pluck` |
| `make test` | Run the unit tests |
| `make test-integration` | Build the binary and run it against the fixture vault in `testdata/vault` |
| `make vet` | Run `go vet`, including the integration tests |
| `make fmt` | Format the code |
| `make check` | Everything CI runs: format check, vet, unit and integration tests |

Code layout:

| Path | Contents |
|---|---|
| `cmd/` | The command-line interface (Cobra) and config loading (Viper) |
| `internal/filter/` | The directory, file name and frontmatter filters |
| `internal/frontmatter/` | Reading YAML frontmatter from markdown files |
| `internal/vault/` | Walking the source directory and copying files |
| `integration_test.go` | Tests that run the built binary (`integration` build tag) |
| `testdata/` | The fixture vault and config files used by the integration tests |

CI runs `make check` and `make build` on every push to `main` and on pull
requests.

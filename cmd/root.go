// Package cmd implements the pluck command line interface.
package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"pluck/internal/filter"
	"pluck/internal/vault"
)

// Config holds the search parameters, loaded from a YAML file and/or flags.
type Config struct {
	Source  string
	Target  string
	Copy    bool
	Filters FiltersConfig
}

// FiltersConfig lists the patterns to match. A file is selected when any
// pattern matches.
type FiltersConfig struct {
	Directories []string
	Filenames   []string
	Frontmatter []FrontmatterRule
}

// FrontmatterRule matches a glob against one frontmatter field.
type FrontmatterRule struct {
	Field   string
	Pattern string
}

// filterFlags holds the filter flag values. They are read from these
// variables rather than through Viper, which splits array flags on commas,
// or GetStringArray, which drops a single empty pattern.
type filterFlags struct {
	dirs, names, frontmatter []string
}

// NewRootCmd builds the root command.
func NewRootCmd() *cobra.Command {
	v := viper.New()
	var configFile string
	var ff filterFlags

	c := &cobra.Command{
		Use:   "pluck",
		Short: "Pluck matching files out of a directory tree and copy them elsewhere",
		Long: `Find files of any type in a directory tree, such as an Obsidian vault,
by directory, file name or markdown frontmatter field, all matched with glob
patterns. A file is selected when any filter matches. Frontmatter is only
read from markdown (.md) files, and hidden files and folders such as
.obsidian are skipped.

Patterns must match the whole value and support * (anything except /),
? (one character), [abc], {a,b} and ** (any number of folders).

By default the matching files are only listed (dry run). Pass --copy to copy
them to the target directory, keeping their folder structure.`,
		Example: `  pluck --config sync.yaml
  pluck -s ~/vault --dir 'Blog/**' --frontmatter 'tags=publish'
  pluck -s ~/vault -t ./out --name '[0-9][0-9][0-9][0-9]-*.md' --copy`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, _ []string) error {
			cfg, err := loadConfig(c, v, configFile, ff)
			if err != nil {
				return err
			}
			return run(c, cfg)
		},
	}

	flags := c.Flags()
	flags.StringVarP(&configFile, "config", "c", "", "YAML config file with the search parameters")
	flags.StringP("source", "s", "", "vault directory to search")
	flags.StringP("target", "t", "", "directory to copy matching files into")
	flags.Bool("copy", false, "copy matching files to the target (default is a dry run)")
	flags.StringArrayVar(&ff.dirs, "dir", nil, "glob matched against the file's directory relative to the vault, e.g. Blog/** (repeatable)")
	flags.StringArrayVar(&ff.names, "name", nil, "glob matched against the file name, e.g. *-draft.md (repeatable)")
	flags.StringArrayVar(&ff.frontmatter, "frontmatter", nil, "field=glob matched against a frontmatter field, e.g. tags=proj-*; list fields match on any element (repeatable)")

	for _, key := range []string{"source", "target", "copy"} {
		_ = v.BindPFlag(key, flags.Lookup(key))
	}
	return c
}

// loadConfig merges the config file with flags. Flags that are set replace
// the matching config values.
func loadConfig(c *cobra.Command, v *viper.Viper, configFile string, ff filterFlags) (Config, error) {
	var cfg Config
	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return cfg, fmt.Errorf("read config: %w", err)
		}
	}
	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	flags := c.Flags()
	if flags.Changed("dir") {
		cfg.Filters.Directories = ff.dirs
	}
	if flags.Changed("name") {
		cfg.Filters.Filenames = ff.names
	}
	if flags.Changed("frontmatter") {
		cfg.Filters.Frontmatter = nil
		for _, value := range ff.frontmatter {
			field, pattern, ok := strings.Cut(value, "=")
			if !ok || field == "" {
				return cfg, fmt.Errorf("--frontmatter %q must be in the form field=pattern", value)
			}
			cfg.Filters.Frontmatter = append(cfg.Filters.Frontmatter, FrontmatterRule{Field: field, Pattern: pattern})
		}
	}

	var err error
	if cfg.Source, err = expandHome(cfg.Source); err != nil {
		return cfg, err
	}
	if cfg.Target, err = expandHome(cfg.Target); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func run(c *cobra.Command, cfg Config) error {
	if cfg.Source == "" {
		return errors.New("a source vault is required (--source or source: in the config)")
	}
	if err := checkSource(cfg.Source); err != nil {
		return err
	}
	match, err := buildFilter(cfg.Filters)
	if err != nil {
		return err
	}
	if cfg.Copy && cfg.Target == "" {
		return errors.New("a target directory is required to copy (--target or target: in the config)")
	}
	if cfg.Target != "" {
		if err := checkTarget(cfg.Source, cfg.Target); err != nil {
			return err
		}
	}

	files, warnings, err := vault.Scan(cfg.Source)
	if err != nil {
		return fmt.Errorf("scan vault: %w", err)
	}
	for _, w := range warnings {
		fmt.Fprintf(c.ErrOrStderr(), "warning: %v\n", w)
	}

	var selected []string
	for _, f := range files {
		if match.Match(f) {
			selected = append(selected, f.RelPath)
			fmt.Fprintln(c.OutOrStdout(), f.RelPath)
		}
	}

	if !cfg.Copy {
		fmt.Fprintf(c.ErrOrStderr(), "%d file(s) matched (dry run, pass --copy to copy them)\n", len(selected))
		return nil
	}
	if err := vault.Copy(cfg.Source, cfg.Target, selected); err != nil {
		return err
	}
	fmt.Fprintf(c.ErrOrStderr(), "copied %d file(s) to %s\n", len(selected), cfg.Target)
	return nil
}

func buildFilter(cfg FiltersConfig) (filter.Any, error) {
	var filters filter.Any
	for _, p := range cfg.Directories {
		if err := filter.Validate(p); err != nil {
			return nil, fmt.Errorf("invalid directory pattern %q: %w", p, err)
		}
		filters = append(filters, filter.Directory{Pattern: p})
	}
	for _, p := range cfg.Filenames {
		if err := filter.Validate(p); err != nil {
			return nil, fmt.Errorf("invalid name pattern %q: %w", p, err)
		}
		filters = append(filters, filter.Name{Pattern: p})
	}
	for _, rule := range cfg.Frontmatter {
		if rule.Field == "" {
			return nil, errors.New("frontmatter filter is missing a field")
		}
		if err := filter.Validate(rule.Pattern); err != nil {
			return nil, fmt.Errorf("invalid frontmatter pattern %q for %q: %w", rule.Pattern, rule.Field, err)
		}
		filters = append(filters, filter.Frontmatter{Field: rule.Field, Pattern: rule.Pattern})
	}
	if len(filters) == 0 {
		return nil, errors.New("at least one filter is required (--dir, --name or --frontmatter)")
	}
	return filters, nil
}

func checkSource(source string) error {
	info, err := os.Stat(source)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("source %s does not exist", source)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source %s is not a directory", source)
	}
	return nil
}

// checkTarget rejects a target that is an existing file, and stops copies
// into the vault, which would be picked up again by the next scan. A target
// that does not exist yet is created when copying.
func checkTarget(source, target string) error {
	info, err := os.Stat(target)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("target %s is not a directory", target)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	absSource, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absSource, absTarget)
	if err != nil {
		return err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("target %s is inside the source vault", target)
	}
	return nil
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}

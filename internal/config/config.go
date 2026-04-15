// Package config loads, validates, and canonicalizes imghost's TOML config.
//
// Source of truth: the single file at xdg.ConfigFile("imghost/config.toml").
// No env-variable overrides and no --config flag — the daemon and the CLI
// must read the exact same file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/reserved"
)

// Config is the fully validated runtime configuration.
type Config struct {
	ListenAddr    string            `toml:"listen_addr"`
	APIKey        string            `toml:"api_key"`
	DefaultAccess permission.Access `toml:"default_access"`
	StateDir      string            `toml:"state_dir"`
	Roots         []Root            `toml:"root"`

	// DBPath is derived during Load; it is not a TOML field.
	DBPath string `toml:"-"`
}

const (
	defaultListenAddr    = ":34286"
	defaultAPIKey        = "change-me"
	defaultAccessDefault = permission.Public
)

// Load reads, parses, and validates the TOML config. Any REQ-BO05 failure
// returns an error without side effects beyond what xdg.ConfigFile /
// xdg.StateFile may create (see design.md known side effects).
func Load() (*Config, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var cfg Config
	dec := toml.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	applyDefaults(&cfg)

	if err := expandAndValidate(&cfg); err != nil {
		return nil, err
	}

	dbPath, err := resolveDBPath(cfg.StateDir)
	if err != nil {
		return nil, err
	}
	cfg.DBPath = dbPath

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	if cfg.APIKey == "" {
		cfg.APIKey = defaultAPIKey
	}
	if cfg.DefaultAccess == "" {
		cfg.DefaultAccess = defaultAccessDefault
	}
}

func expandAndValidate(cfg *Config) error {
	if _, err := permission.Parse(string(cfg.DefaultAccess)); err != nil {
		return fmt.Errorf("default_access: %w", err)
	}

	// state_dir: optional; when set, expand ~ and require absolute.
	if cfg.StateDir != "" {
		expanded, err := expandTilde(cfg.StateDir)
		if err != nil {
			return fmt.Errorf("state_dir: expand ~: %w", err)
		}
		if !filepath.IsAbs(expanded) {
			return fmt.Errorf("state_dir %q: must be absolute after ~ expansion", cfg.StateDir)
		}
		cfg.StateDir = expanded
	}

	if len(cfg.Roots) == 0 {
		return errors.New("at least one [[root]] is required")
	}
	seen := make(map[string]struct{}, len(cfg.Roots))
	for i := range cfg.Roots {
		r := &cfg.Roots[i]
		if err := validateRoot(i, r); err != nil {
			return err
		}
		if _, dup := seen[r.Name]; dup {
			return fmt.Errorf("root[%d] name %q: duplicate", i, r.Name)
		}
		seen[r.Name] = struct{}{}
	}
	return nil
}

func validateRoot(i int, r *Root) error {
	if r.Name == "" {
		return fmt.Errorf("root[%d] name: required", i)
	}
	if r.Name == "." || r.Name == ".." {
		return fmt.Errorf("root[%d] name %q: invalid", i, r.Name)
	}
	for _, c := range r.Name {
		if c == '/' {
			return fmt.Errorf("root[%d] name %q: must not contain '/'", i, r.Name)
		}
	}
	if reserved.IsName(r.Name) {
		return fmt.Errorf("root[%d] name %q: conflicts with reserved prefix", i, r.Name)
	}
	if r.Access != "" {
		if _, err := permission.Parse(string(r.Access)); err != nil {
			return fmt.Errorf("root[%d] access: %w", i, err)
		}
	}
	if r.Path == "" {
		return fmt.Errorf("root[%d] path: required", i)
	}
	expanded, err := expandTilde(r.Path)
	if err != nil {
		return fmt.Errorf("root[%d] path: expand ~: %w", i, err)
	}
	if !filepath.IsAbs(expanded) {
		return fmt.Errorf("root[%d] path %q: must be absolute after ~ expansion", i, r.Path)
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return fmt.Errorf("root[%d] path %q: %w", i, r.Path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root[%d] path %q: not a directory", i, r.Path)
	}
	r.Path = expanded
	return nil
}

func resolveDBPath(stateDir string) (string, error) {
	if stateDir == "" {
		return defaultStateDBPath()
	}
	return filepath.Join(stateDir, StateDBName), nil
}

// RootByName returns the root registered under name. Names are compared
// case-sensitively and must match exactly.
func (c *Config) RootByName(name string) (*Root, bool) {
	for i := range c.Roots {
		if c.Roots[i].Name == name {
			return &c.Roots[i], true
		}
	}
	return nil, false
}

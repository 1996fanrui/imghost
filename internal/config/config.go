// Package config loads, validates, and canonicalizes imghost's TOML config.
//
// Source of truth: the single file at xdg.ConfigFile("imghost/config.toml").
// No env-variable overrides and no --config flag — the daemon and the CLI
// must read the exact same file.
package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"

	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/reserved"
)

// defaultRootName is the reserved name used when no [[root]] is configured.
const defaultRootName = "_default"

// Config is the fully validated runtime configuration.
type Config struct {
	ListenAddr    string            `toml:"listen_addr"`
	APIKey        string            `toml:"api_key"`
	DefaultAccess permission.Access `toml:"default_access"`
	StateDir      string            `toml:"state_dir"`
	Roots         []Root            `toml:"root"`

	// DBPath is derived during Load; it is not a TOML field.
	DBPath string `toml:"-"`

	// DefaultRootInjected is true when Load synthesized the built-in
	// `_default` root because the user configured no [[root]]. The daemon
	// logs this once at startup; the CLI stays silent.
	DefaultRootInjected bool `toml:"-"`

	// APIKeyGenerated is true when Load generated a fresh api_key during
	// first-run config bootstrap. The daemon logs a one-time notice at
	// startup; the CLI stays silent.
	APIKeyGenerated bool `toml:"-"`
}

const (
	defaultListenAddr    = "127.0.0.1:34286"
	defaultAccessDefault = permission.Public

	// apiKeyRandBytes is the size of the raw entropy used for the generated
	// api_key. 32 bytes = 256 bits, rendered as 43 base64url characters.
	apiKeyRandBytes = 32
)

// Load reads, parses, and validates the TOML config. It may create
// directories and files on first run: xdg.ConfigFile / xdg.StateFile may
// create parent directories, and when the config file is absent Load
// writes a freshly generated minimal config.toml with 0600 permissions.
func Load() (*Config, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	var cfg Config
	f, err := os.Open(path)
	switch {
	case err == nil:
		defer func() { _ = f.Close() }()
		dec := toml.NewDecoder(f)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	case errors.Is(err, os.ErrNotExist):
		// First run: materialize a minimal config with a freshly generated
		// api_key so the default install has a per-host shared secret
		// instead of a well-known one. Subsequent Load calls read the file
		// like any other run.
		key, genErr := generateAPIKey()
		if genErr != nil {
			return nil, fmt.Errorf("generate api_key: %w", genErr)
		}
		if err := writeBootstrapConfig(path, key); err != nil {
			return nil, err
		}
		cfg.ListenAddr = defaultListenAddr
		cfg.APIKey = key
		cfg.APIKeyGenerated = true
	default:
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}

	if !cfg.APIKeyGenerated && cfg.APIKey == "" {
		return nil, fmt.Errorf("config %s: api_key must be set (remove the file to regenerate)", path)
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
	if len(cfg.Roots) == 0 {
		return injectDefaultRoot(cfg)
	}
	return nil
}

// injectDefaultRoot appends a public "_default" root backed by the XDG data
// directory when the user has not configured any [[root]]. The directory is
// created on demand so a brand-new install is immediately usable.
func injectDefaultRoot(cfg *Config) error {
	path, err := xdg.DataFile("imghost/data")
	if err != nil {
		return fmt.Errorf("resolve default root path: %w", err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create default root %s: %w", path, err)
	}
	cfg.Roots = append(cfg.Roots, Root{
		Name:   defaultRootName,
		Path:   path,
		Access: permission.Public,
	})
	cfg.DefaultRootInjected = true
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
	if r.Name == defaultRootName {
		return fmt.Errorf("root[%d] name %q: reserved for default root injection", i, r.Name)
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

// generateAPIKey returns a base64url-encoded random token backed by
// crypto/rand. The padding is stripped so the value is a clean bearer token.
func generateAPIKey() (string, error) {
	buf := make([]byte, apiKeyRandBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// writeBootstrapConfig atomically writes a minimal config.toml with
// 0600 permissions on first run. The parent directory is created by
// xdg.ConfigFile before this is called.
func writeBootstrapConfig(path, apiKey string) error {
	contents := fmt.Sprintf("listen_addr = %q\napi_key = %q\n", defaultListenAddr, apiKey)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.toml.*")
	if err != nil {
		return fmt.Errorf("create bootstrap config: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod bootstrap config: %w", err)
	}
	if _, err := tmp.WriteString(contents); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write bootstrap config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close bootstrap config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("install bootstrap config %s: %w", path, err)
	}
	return nil
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

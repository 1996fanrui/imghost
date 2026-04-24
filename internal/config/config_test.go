package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"

	"github.com/1996fanrui/filehub/internal/permission"
)

func reloadXDG() { xdg.Reload() }

// writeConfig sets XDG_CONFIG_HOME + XDG_STATE_HOME under a temp dir and
// writes the supplied TOML as the filehub config. Returns the config path.
func writeConfig(t *testing.T, toml string) string {
	t.Helper()
	cfgHome := t.TempDir()
	stateHome := t.TempDir()
	dataHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_DATA_HOME", dataHome)
	// adrg/xdg reads env lazily on first call. Force a reload per test.
	reloadXDG()
	dir := filepath.Join(cfgHome, "filehub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Inject a default api_key when the TOML body omits one; most tests
	// are focused on other fields and should not have to spell it out.
	if !strings.Contains(toml, "api_key") {
		toml = "api_key = \"test-key\"\n" + toml
	}
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_Defaults(t *testing.T) {
	rootDir := t.TempDir()
	writeConfig(t, `
[[root]]
name = "photos"
path = "`+rootDir+`"
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:34286" {
		t.Errorf("listen_addr = %q", cfg.ListenAddr)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("api_key = %q", cfg.APIKey)
	}
	if cfg.DefaultAccess != permission.Public {
		t.Errorf("default_access = %v", cfg.DefaultAccess)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0].Name != "photos" {
		t.Fatalf("roots = %+v", cfg.Roots)
	}
	if cfg.Roots[0].Path != rootDir {
		t.Errorf("root path = %q want %q", cfg.Roots[0].Path, rootDir)
	}
	if !strings.HasSuffix(cfg.DBPath, "filehub.db") {
		t.Errorf("db path = %q", cfg.DBPath)
	}
}

func TestLoad_StrictUnknownField(t *testing.T) {
	rootDir := t.TempDir()
	writeConfig(t, `
bogus = "x"
[[root]]
name = "photos"
path = "`+rootDir+`"
`)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for unknown top-level key")
	}
}

func TestLoad_StrictUnknownNestedField(t *testing.T) {
	rootDir := t.TempDir()
	writeConfig(t, `
[[root]]
name = "photos"
path = "`+rootDir+`"
extra = "nope"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error for unknown nested key")
	}
}

func TestLoad_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	sub := filepath.Join(home, ".filehub-test-tilde")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(sub) })
	writeConfig(t, `
[[root]]
name = "photos"
path = "~/.filehub-test-tilde"
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Roots[0].Path != sub {
		t.Fatalf("path = %q want %q", cfg.Roots[0].Path, sub)
	}
}

func TestLoad_NoRoots(t *testing.T) {
	writeConfig(t, ``)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roots) != 1 {
		t.Fatalf("roots = %+v", cfg.Roots)
	}
	r := cfg.Roots[0]
	if r.Name != "_default" {
		t.Errorf("name = %q", r.Name)
	}
	if r.Access != permission.Public {
		t.Errorf("access = %v", r.Access)
	}
	info, err := os.Stat(r.Path)
	if err != nil {
		t.Fatalf("stat default root path: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("default root path %q is not a directory", r.Path)
	}
}

func TestLoad_DefaultNameRejected(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
[[root]]
name = "_default"
path = "`+d+`"
`)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "reserved for default root injection") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoad_DuplicateRootName(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
[[root]]
name = "a"
path = "`+d+`"
[[root]]
name = "a"
path = "`+d+`"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_RootNameEmpty(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
[[root]]
name = ""
path = "`+d+`"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_RootNameContainsSlash(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
[[root]]
name = "a/b"
path = "`+d+`"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_RootNameDot(t *testing.T) {
	d := t.TempDir()
	for _, n := range []string{".", ".."} {
		writeConfig(t, `
[[root]]
name = "`+n+`"
path = "`+d+`"
`)
		if _, err := Load(); err == nil {
			t.Fatalf("expected error for name=%q", n)
		}
	}
}

func TestLoad_RootNameReserved(t *testing.T) {
	d := t.TempDir()
	for _, n := range []string{"swagger"} {
		writeConfig(t, `
[[root]]
name = "`+n+`"
path = "`+d+`"
`)
		if _, err := Load(); err == nil {
			t.Fatalf("expected error for reserved name %q", n)
		}
	}
}

func TestLoad_RootPathRelative(t *testing.T) {
	writeConfig(t, `
[[root]]
name = "a"
path = "relative/path"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_RootPathMissing(t *testing.T) {
	writeConfig(t, `
[[root]]
name = "a"
path = "/definitely/does/not/exist/xyz"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_RootPathIsFile(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "a_file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, `
[[root]]
name = "a"
path = "`+f+`"
`)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when root.path is a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoad_ConfigMissing(t *testing.T) {
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	reloadXDG()
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.APIKeyGenerated {
		t.Fatal("APIKeyGenerated = false, want true on first run")
	}
	if cfg.APIKey == "" {
		t.Fatal("generated api_key is empty")
	}
	if cfg.ListenAddr != "127.0.0.1:34286" {
		t.Errorf("listen_addr = %q, want loopback", cfg.ListenAddr)
	}
	// Bootstrap config must be persisted with 0600 perms.
	cfgPath := filepath.Join(cfgHome, "filehub", "config.toml")
	st, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat bootstrap config: %v", err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("bootstrap config perm = %#o, want 0600", perm)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0].Name != "_default" {
		t.Fatalf("roots = %+v", cfg.Roots)
	}
	if cfg.Roots[0].Access != permission.Public {
		t.Errorf("access = %v", cfg.Roots[0].Access)
	}
	info, err := os.Stat(cfg.Roots[0].Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("not a dir: %q", cfg.Roots[0].Path)
	}
}

func TestLoad_ConfigMissing_GeneratesUniquePerHost(t *testing.T) {
	loadFresh := func() string {
		t.Helper()
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		reloadXDG()
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		return cfg.APIKey
	}
	a, b := loadFresh(), loadFresh()
	if a == b {
		t.Fatalf("two fresh bootstraps produced the same api_key %q", a)
	}
}

func TestLoad_EmptyAPIKeyRejected(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
api_key = ""
[[root]]
name = "photos"
path = "`+d+`"
`)
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for empty api_key")
	}
	if !strings.Contains(err.Error(), "api_key must be set") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoad_BadDefaultAccess(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
default_access = "secret"
[[root]]
name = "a"
path = "`+d+`"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_BadRootAccess(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
[[root]]
name = "a"
path = "`+d+`"
access = "weird"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_StateDirRelative(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
state_dir = "relative"
[[root]]
name = "a"
path = "`+d+`"
`)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_StateDirAbsolute(t *testing.T) {
	d := t.TempDir()
	stateDir := t.TempDir()
	writeConfig(t, `
state_dir = "`+stateDir+`"
[[root]]
name = "a"
path = "`+d+`"
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBPath != filepath.Join(stateDir, "filehub.db") {
		t.Fatalf("db path = %q", cfg.DBPath)
	}
}

func TestLoad_XDGStateDefault(t *testing.T) {
	d := t.TempDir()
	stateHome := t.TempDir()
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	reloadXDG()
	// write config after setting xdg so reloadXDG sees it
	dir := filepath.Join(cfgHome, "filehub")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
api_key = "test-key"
[[root]]
name = "a"
path = "`+d+`"
`), 0o644)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(stateHome, "filehub", "filehub.db")
	if cfg.DBPath != want {
		t.Fatalf("db path = %q want %q", cfg.DBPath, want)
	}
}

func TestRootByName(t *testing.T) {
	d := t.TempDir()
	writeConfig(t, `
[[root]]
name = "a"
path = "`+d+`"
[[root]]
name = "b"
path = "`+d+`"
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.RootByName("a"); !ok {
		t.Fatal("missing a")
	}
	if _, ok := cfg.RootByName("c"); ok {
		t.Fatal("unexpected c")
	}
}

func TestEffectiveAccess(t *testing.T) {
	r := Root{}
	if got := r.EffectiveAccess(permission.Private); got != permission.Private {
		t.Fatalf("got %v", got)
	}
	r.Access = permission.Public
	if got := r.EffectiveAccess(permission.Private); got != permission.Public {
		t.Fatalf("got %v", got)
	}
}

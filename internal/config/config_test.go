package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrg/xdg"

	"github.com/1996fanrui/imghost/internal/permission"
)

func reloadXDG() { xdg.Reload() }

// writeConfig sets XDG_CONFIG_HOME + XDG_STATE_HOME under a temp dir and
// writes the supplied TOML as the imghost config. Returns the config path.
func writeConfig(t *testing.T, toml string) string {
	t.Helper()
	cfgHome := t.TempDir()
	stateHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	// adrg/xdg reads env lazily on first call. Force a reload per test.
	reloadXDG()
	dir := filepath.Join(cfgHome, "imghost")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
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
	if cfg.ListenAddr != ":34286" {
		t.Errorf("listen_addr = %q", cfg.ListenAddr)
	}
	if cfg.APIKey != "change-me" {
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
	if !strings.HasSuffix(cfg.DBPath, "imghost.db") {
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
	sub := filepath.Join(home, ".imghost-test-tilde")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(sub) })
	writeConfig(t, `
[[root]]
name = "photos"
path = "~/.imghost-test-tilde"
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
	if _, err := Load(); err == nil {
		t.Fatal("expected error for zero roots")
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
	reloadXDG()
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
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
	if cfg.DBPath != filepath.Join(stateDir, "imghost.db") {
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
	dir := filepath.Join(cfgHome, "imghost")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
[[root]]
name = "a"
path = "`+d+`"
`), 0o644)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(stateHome, "imghost", "imghost.db")
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

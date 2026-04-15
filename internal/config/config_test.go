package config

import (
	"os"
	"runtime"
	"testing"

	"github.com/1996fanrui/imghost/internal/permission"
)

func setBase(t *testing.T, dataDir string) {
	t.Helper()
	t.Setenv("API_KEY", "k")
	t.Setenv("DEFAULT_ACCESS", "public")
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("PORT", "")
}

func TestLoad_Happy(t *testing.T) {
	setBase(t, t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.APIKey != "k" || c.DefaultAccess != permission.Public || c.Port != 34286 {
		t.Fatalf("got %+v", c)
	}
	if c.DBPath != DefaultDBPath {
		t.Fatalf("db path %q", c.DBPath)
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	setBase(t, t.TempDir())
	t.Setenv("API_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_BadDefaultAccess(t *testing.T) {
	setBase(t, t.TempDir())
	t.Setenv("DEFAULT_ACCESS", "secret")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DefaultAccessPrivate(t *testing.T) {
	setBase(t, t.TempDir())
	t.Setenv("DEFAULT_ACCESS", "private")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultAccess != permission.Private {
		t.Fatalf("got %v", c.DefaultAccess)
	}
}

func TestLoad_BadPort(t *testing.T) {
	for _, v := range []string{"abc", "0", "70000", "-1"} {
		t.Run(v, func(t *testing.T) {
			setBase(t, t.TempDir())
			t.Setenv("PORT", v)
			if _, err := Load(); err == nil {
				t.Fatalf("PORT=%q: expected error", v)
			}
		})
	}
}

func TestLoad_DataDirMissing(t *testing.T) {
	setBase(t, "/definitely/does/not/exist/xyz")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DataDirReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod-based read-only test requires unix non-root")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	setBase(t, dir)
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DataDirReservedExact(t *testing.T) {
	setBase(t, "/var/lib/imghost")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_DataDirReservedUnder(t *testing.T) {
	setBase(t, "/var/lib/imghost/sub")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

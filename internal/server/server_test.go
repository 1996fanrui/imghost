package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/1996fanrui/imghost/internal/config"
	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/storage"
)

type testEnv struct {
	ts        *httptest.Server
	dataDir   string
	permstore *storage.PermStore
	fs        storage.FS
	apiKey    string
}

// testServer builds a server wired against a temp dataDir + temp bbolt with
// the provided FS (pass nil for OSFS) and default access. Caller may mutate
// env.fs for injection-style tests by swapping it out before calling — in
// that case use newTestEnvWithFS explicitly.
func testServer(t *testing.T) *testEnv {
	t.Helper()
	return newTestEnvWithFS(t, storage.OSFS{}, permission.Public)
}

func newTestEnvWithFS(t *testing.T, fs storage.FS, defaultAccess permission.Access) *testEnv {
	t.Helper()
	dataDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "imghost.db")
	ps, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open permstore: %v", err)
	}
	cfg := &config.Config{
		APIKey:        "secret",
		DefaultAccess: defaultAccess,
		DataDir:       dataDir,
		Port:          0,
		DBPath:        dbPath,
	}
	handler := New(cfg, fs, ps)
	ts := httptest.NewServer(handler)
	env := &testEnv{
		ts:        ts,
		dataDir:   dataDir,
		permstore: ps,
		fs:        fs,
		apiKey:    cfg.APIKey,
	}
	t.Cleanup(func() {
		ts.Close()
		_ = ps.Close()
	})
	return env
}

package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/1996fanrui/filehub/internal/config"
	"github.com/1996fanrui/filehub/internal/permission"
	"github.com/1996fanrui/filehub/internal/storage"
)

// testRootName is the URL name used for the single root in every test fixture.
const testRootName = "testroot"

type testEnv struct {
	ts        *httptest.Server
	rootPath  string
	permstore *storage.PermStore
	fs        storage.FS
	apiKey    string
}

// newTestServer constructs a single-root fixture rooted at t.TempDir() and
// returns the running httptest server + shared handles. Pass storage.OSFS{}
// for normal disk-backed tests, or a wrapper FS for injection tests.
func newTestServer(t *testing.T, fs storage.FS) *testEnv {
	t.Helper()
	rootPath := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "filehub.db")
	ps, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open permstore: %v", err)
	}
	cfg := &config.Config{
		ListenAddr:    ":0",
		APIKey:        "secret",
		DefaultAccess: permission.Public,
		DBPath:        dbPath,
		Roots: []config.Root{
			{Name: testRootName, Path: rootPath},
		},
	}
	handler := New(cfg, fs, ps)
	ts := httptest.NewServer(handler)
	env := &testEnv{
		ts:        ts,
		rootPath:  rootPath,
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

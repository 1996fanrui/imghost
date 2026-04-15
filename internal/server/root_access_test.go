package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/1996fanrui/imghost/internal/config"
	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/storage"
)

// newTwoRootServer builds a server with two roots whose `access` fields are
// caller-controlled, so tests can assert that per-root `access` overrides
// (REQ-2QFE) actually flow into the permission resolver at request time.
func newTwoRootServer(t *testing.T, defaultAccess, photosAccess, docsAccess permission.Access) (*httptest.Server, string, string) {
	t.Helper()
	photosPath := t.TempDir()
	docsPath := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "imghost.db")
	ps, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open permstore: %v", err)
	}
	cfg := &config.Config{
		ListenAddr:    ":0",
		APIKey:        "secret",
		DefaultAccess: defaultAccess,
		DBPath:        dbPath,
		Roots: []config.Root{
			{Name: "photos", Path: photosPath, Access: photosAccess},
			{Name: "docs", Path: docsPath, Access: docsAccess},
		},
	}
	handler := New(cfg, storage.OSFS{}, ps)
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
		_ = ps.Close()
	})
	if err := os.WriteFile(filepath.Join(photosPath, "x.jpg"), []byte("photo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsPath, "y.md"), []byte("doc"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ts, "photos", "docs"
}

// TestPerRootAccess_OverridesGlobalDefault asserts that a root whose
// access=private blocks unauthenticated GETs while a sibling root with no
// override still inherits the (public) global default.
func TestPerRootAccess_OverridesGlobalDefault(t *testing.T) {
	ts, _, _ := newTwoRootServer(t, permission.Public, permission.Private, "")

	// photos/x.jpg is under a private root — unauthenticated GET must fail.
	resp := doReq(t, "GET", ts.URL+"/photos/x.jpg", nil, nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("photos (private root) unauth GET: got %d, want 401", resp.StatusCode)
	}

	// docs inherits public default — unauthenticated GET must succeed.
	resp = doReq(t, "GET", ts.URL+"/docs/y.md", nil, nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("docs (inherits public default) unauth GET: got %d, want 200", resp.StatusCode)
	}
}

// TestPerRootAccess_FlippedDefault asserts the inverse: with default_access
// flipped to private, a sibling root explicitly marked public stays open
// while the inheriting root becomes auth-blocked.
func TestPerRootAccess_FlippedDefault(t *testing.T) {
	ts, _, _ := newTwoRootServer(t, permission.Private, permission.Public, "")

	// photos is explicitly public — unauthenticated GET must succeed.
	resp := doReq(t, "GET", ts.URL+"/photos/x.jpg", nil, nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("photos (explicit public root) unauth GET: got %d, want 200", resp.StatusCode)
	}

	// docs inherits private default — unauthenticated GET must fail.
	resp = doReq(t, "GET", ts.URL+"/docs/y.md", nil, nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("docs (inherits private default) unauth GET: got %d, want 401", resp.StatusCode)
	}
}

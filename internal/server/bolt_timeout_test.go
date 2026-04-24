package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/1996fanrui/filehub/internal/config"
	"github.com/1996fanrui/filehub/internal/permission"
)

// TestBboltTimeoutFailFast verifies that Start surfaces a bbolt lock
// contention as a fail-fast error (REQ-BO05) instead of hanging. We
// shrink boltOpenTimeout so the test runs in well under a second; a
// sibling bolt.Open holds the exclusive lock for the duration.
func TestBboltTimeoutFailFast(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "filehub.db")

	// Acquire the lock first so Start cannot open the file.
	locker, err := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	defer func() { _ = locker.Close() }()

	orig := boltOpenTimeout
	boltOpenTimeout = 200 * time.Millisecond
	defer func() { boltOpenTimeout = orig }()

	cfg := &config.Config{
		ListenAddr:    "127.0.0.1:0",
		APIKey:        "k",
		DefaultAccess: permission.Public,
		DBPath:        dbPath,
		Roots: []config.Root{
			{Name: "photos", Path: dir},
		},
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	err = Start(ctx, cfg)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected bbolt timeout error, got nil")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Start returned after %v; expected fail-fast under 5s", elapsed)
	}
	if !strings.Contains(err.Error(), dbPath) {
		t.Fatalf("error does not mention db path %q: %v", dbPath, err)
	}
}

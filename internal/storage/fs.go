package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FS abstracts the filesystem operations used by handlers. It exists so tests
// can inject failures (rename, write) to verify AT-W81S atomicity semantics
// without touching real syscalls.
type FS interface {
	Stat(absPath string) (os.FileInfo, error)
	Open(absPath string) (io.ReadSeekCloser, error)
	// AtomicWrite streams body into a same-directory temp file, fsyncs it,
	// then renames it onto absPath. On any failure the temp file is removed.
	AtomicWrite(absPath string, body io.Reader) error
	Remove(absPath string) error
}

// DefaultFS is swapped out by tests to inject failures.
var DefaultFS FS = OSFS{}

type OSFS struct{}

func (OSFS) Stat(p string) (os.FileInfo, error) { return os.Stat(p) }

func (OSFS) Open(p string) (io.ReadSeekCloser, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (OSFS) Remove(p string) error { return os.Remove(p) }

func (OSFS) AtomicWrite(absPath string, body io.Reader) error {
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpName)
	}
	if _, err := io.Copy(tmp, body); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("copy body: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, absPath); err != nil {
		cleanup()
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ErrInjected is a sentinel error useful for FS fakes in tests.
var ErrInjected = errors.New("injected failure")

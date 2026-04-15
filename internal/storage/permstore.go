// Package storage contains persistence primitives backed by bbolt.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bolt "go.etcd.io/bbolt"

	"github.com/1996fanrui/imghost/internal/permission"
)

var bucketName = []byte("permissions")

type PermStore struct {
	db *bolt.DB
}

// Open opens the bbolt database at path, creating the parent directory and
// the permissions bucket if needed. Equivalent to OpenWithOptions(path, nil).
func Open(path string) (*PermStore, error) {
	return OpenWithOptions(path, nil)
}

// OpenWithOptions behaves like Open but forwards bolt options (e.g. Timeout)
// so callers can classify a stuck lock as a fail-fast startup error.
func OpenWithOptions(path string, opts *bolt.Options) (*PermStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create bbolt parent dir: %w", err)
	}
	db, err := bolt.Open(path, 0o600, opts)
	if err != nil {
		return nil, fmt.Errorf("open bbolt: %w", err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, e := tx.CreateBucketIfNotExists(bucketName)
		return e
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensure bucket: %w", err)
	}
	return &PermStore{db: db}, nil
}

func (s *PermStore) Close() error { return s.db.Close() }

func (s *PermStore) Get(path string) (permission.Access, bool, error) {
	key, err := normalizeKey(path)
	if err != nil {
		return "", false, err
	}
	var out permission.Access
	var found bool
	err = s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketName).Get([]byte(key))
		if v == nil {
			return nil
		}
		a, perr := permission.Parse(string(v))
		if perr != nil {
			return fmt.Errorf("corrupt access value at %q: %w", key, perr)
		}
		out = a
		found = true
		return nil
	})
	return out, found, err
}

func (s *PermStore) Put(path string, a permission.Access) error {
	key, err := normalizeKey(path)
	if err != nil {
		return err
	}
	if _, err := permission.Parse(string(a)); err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(key), []byte(a))
	})
}

func (s *PermStore) Delete(path string) error {
	key, err := normalizeKey(path)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Delete([]byte(key))
	})
}

func normalizeKey(p string) (string, error) {
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("path %q must start with /", p)
	}
	if p == "/" {
		return p, nil
	}
	return strings.TrimRight(p, "/"), nil
}

// Compile-time assertion that *PermStore satisfies permission.Store.
var _ permission.Store = (*PermStore)(nil)

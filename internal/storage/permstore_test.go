package storage

import (
	"path/filepath"
	"testing"

	"github.com/1996fanrui/imghost/internal/permission"
)

func openTemp(t *testing.T) *PermStore {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "sub", "imghost.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestPermStore_CRUD(t *testing.T) {
	s := openTemp(t)

	if _, ok, err := s.Get("/a"); err != nil || ok {
		t.Fatalf("expected miss, got ok=%v err=%v", ok, err)
	}
	if err := s.Put("/a", permission.Private); err != nil {
		t.Fatal(err)
	}
	a, ok, err := s.Get("/a")
	if err != nil || !ok || a != permission.Private {
		t.Fatalf("got %v, %v, %v", a, ok, err)
	}
	if err := s.Put("/a", permission.Public); err != nil {
		t.Fatal(err)
	}
	a, _, _ = s.Get("/a")
	if a != permission.Public {
		t.Fatalf("overwrite failed: %v", a)
	}
	if err := s.Delete("/a"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Get("/a"); ok {
		t.Fatal("expected deleted")
	}
}

func TestPermStore_Normalization(t *testing.T) {
	s := openTemp(t)
	if err := s.Put("/a/b/", permission.Private); err != nil {
		t.Fatal(err)
	}
	a, ok, err := s.Get("/a/b")
	if err != nil || !ok || a != permission.Private {
		t.Fatalf("got %v ok=%v err=%v", a, ok, err)
	}
}

func TestPermStore_InvalidKey(t *testing.T) {
	s := openTemp(t)
	if err := s.Put("relative", permission.Public); err == nil {
		t.Fatal("expected error for relative path")
	}
}

func TestPermStore_InvalidValue(t *testing.T) {
	s := openTemp(t)
	if err := s.Put("/a", permission.Access("bogus")); err == nil {
		t.Fatal("expected error")
	}
}

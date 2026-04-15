package server

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTraversalSegments(t *testing.T) {
	dir := t.TempDir()
	inputs := []string{
		"/a/../b",
		"/../etc/passwd",
		"/a/%2e%2e/b",
		"/a/%2E%2E/b",
		"/a/%2f%2e%2e%2fb",
		"/..",
		"/a/..",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			if _, _, err := ResolvePath(dir, in); !errors.Is(err, ErrTraversal) {
				t.Fatalf("ResolvePath(%q) err = %v want ErrTraversal", in, err)
			}
		})
	}
}

func TestCleanPath(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"/a//b": "/a/b",
		"/./a":  "/a",
		"/a/./b/":  "/a/b",
		"/":    "/",
	}
	for in, want := range cases {
		cleaned, physical, err := ResolvePath(dir, in)
		if err != nil {
			t.Fatalf("ResolvePath(%q) err = %v", in, err)
		}
		if cleaned != want {
			t.Errorf("cleaned(%q) = %q want %q", in, cleaned, want)
		}
		wantPhys := filepath.Join(dir, want)
		if physical != wantPhys {
			t.Errorf("physical(%q) = %q want %q", in, physical, wantPhys)
		}
	}
}

func TestFilepathRelGuard(t *testing.T) {
	// The segment check (step 2) and path.Clean starting with "/" make it
	// impossible to craft a real input that slips past steps 1-3 yet fails
	// filepath.Rel. Exercise the rel-based guard directly via isWithinRoot
	// so the branch remains covered.
	root := "/tmp/root"
	cases := []struct {
		target string
		want   bool
	}{
		{"/tmp/root/a/b", true},
		{"/tmp/root", true},
		{"/tmp/other", false},
		{"/etc/passwd", false},
	}
	for _, c := range cases {
		if got := isWithinRoot(root, c.target); got != c.want {
			t.Errorf("isWithinRoot(%q,%q) = %v want %v", root, c.target, got, c.want)
		}
	}
}

func TestSymlinkEscapeToTmp(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "escape")
	if err := os.Symlink("/tmp", link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolvePath(dir, "/escape"); !errors.Is(err, ErrSymlinkEscape) {
		t.Fatalf("err = %v want ErrSymlinkEscape", err)
	}
}

func TestSymlinkEscapeToFileOutside(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "hostname")
	if err := os.Symlink("/etc/hostname", link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolvePath(dir, "/hostname"); !errors.Is(err, ErrSymlinkEscape) {
		t.Fatalf("err = %v want ErrSymlinkEscape", err)
	}
}

func TestSymlinkWithinDataDirOK(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "alias.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolvePath(dir, "/alias.txt"); err != nil {
		t.Fatalf("err = %v want nil", err)
	}
}

func TestNonexistentPathOK(t *testing.T) {
	dir := t.TempDir()
	// Parent exists (dir), target does not yet.
	cleaned, physical, err := ResolvePath(dir, "/new/file.png")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if cleaned != "/new/file.png" {
		t.Errorf("cleaned = %q", cleaned)
	}
	if physical != filepath.Join(dir, "new/file.png") {
		t.Errorf("physical = %q", physical)
	}
}

func TestNonexistentUnderEscapingSymlinkParent(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "bad")
	if err := os.Symlink("/tmp", link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ResolvePath(dir, "/bad/new.txt"); !errors.Is(err, ErrSymlinkEscape) {
		t.Fatalf("err = %v want ErrSymlinkEscape", err)
	}
}

package permission

import "testing"

type fakeStore map[string]Access

func (f fakeStore) Get(p string) (Access, bool, error) {
	a, ok := f[p]
	return a, ok, nil
}

func TestResolver_DirectoryInherit(t *testing.T) {
	r := &Resolver{Store: fakeStore{"/a": Private}, Default: Public}
	got, err := r.Resolve("/a/b/c.png")
	if err != nil || got != Private {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_FileOverridesDir(t *testing.T) {
	r := &Resolver{Store: fakeStore{
		"/a":       Private,
		"/a/b.png": Public,
	}, Default: Private}
	got, err := r.Resolve("/a/b.png")
	if err != nil || got != Public {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_NoRuleUsesDefault(t *testing.T) {
	for _, d := range []Access{Public, Private} {
		r := &Resolver{Store: fakeStore{}, Default: d}
		got, err := r.Resolve("/x/y")
		if err != nil || got != d {
			t.Fatalf("default %v: got %v, %v", d, got, err)
		}
	}
}

func TestResolver_TrailingSlash(t *testing.T) {
	r := &Resolver{Store: fakeStore{"/a": Private}, Default: Public}
	got, err := r.Resolve("/a/b/")
	if err != nil || got != Private {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_RootRule(t *testing.T) {
	r := &Resolver{Store: fakeStore{"/": Private}, Default: Public}
	got, err := r.Resolve("/foo")
	if err != nil || got != Private {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_RejectRelative(t *testing.T) {
	r := &Resolver{Store: fakeStore{}, Default: Public}
	if _, err := r.Resolve("foo"); err == nil {
		t.Fatal("expected error")
	}
}

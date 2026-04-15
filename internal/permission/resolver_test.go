package permission

import "testing"

type fakeStore map[string]Access

func (f fakeStore) Get(p string) (Access, bool, error) {
	a, ok := f[p]
	return a, ok, nil
}

func TestResolver_DirectoryInherit(t *testing.T) {
	r := &Resolver{Store: fakeStore{"/a": Private}}
	got, err := r.Resolve("/a/b/c.png", Public)
	if err != nil || got != Private {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_FileOverridesDir(t *testing.T) {
	r := &Resolver{Store: fakeStore{
		"/a":       Private,
		"/a/b.png": Public,
	}}
	got, err := r.Resolve("/a/b.png", Private)
	if err != nil || got != Public {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_NoRuleUsesDefault(t *testing.T) {
	for _, d := range []Access{Public, Private} {
		r := &Resolver{Store: fakeStore{}}
		got, err := r.Resolve("/x/y", d)
		if err != nil || got != d {
			t.Fatalf("default %v: got %v, %v", d, got, err)
		}
	}
}

func TestResolver_TrailingSlash(t *testing.T) {
	r := &Resolver{Store: fakeStore{"/a": Private}}
	got, err := r.Resolve("/a/b/", Public)
	if err != nil || got != Private {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_RootRule(t *testing.T) {
	r := &Resolver{Store: fakeStore{"/": Private}}
	got, err := r.Resolve("/foo", Public)
	if err != nil || got != Private {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestResolver_RejectRelative(t *testing.T) {
	r := &Resolver{Store: fakeStore{}}
	if _, err := r.Resolve("foo", Public); err == nil {
		t.Fatal("expected error")
	}
}

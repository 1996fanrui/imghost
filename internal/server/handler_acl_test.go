package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/1996fanrui/filehub/internal/permission"
	"github.com/1996fanrui/filehub/internal/storage"
)

func TestGetAclExplicit(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", urlOf(env, "/a.txt?acl"), nil, authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["path"] != "/testroot/a.txt" || body["access"] != "private" {
		t.Fatalf("body %+v", body)
	}
}

func TestGetAclNoRule(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "GET", urlOf(env, "/a.txt?acl"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestGetAclNoInheritance(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := env.permstore.Put("/testroot/dir", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", urlOf(env, "/dir/child.txt?acl"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d want 404 (no inheritance)", resp.StatusCode)
	}
}

func TestPutAclPublic(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a.txt?acl"),
		strings.NewReader(`{"access":"public"}`), authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if len(b) != 0 {
		t.Fatalf("body not empty: %q", b)
	}
	a, ok, _ := env.permstore.Get("/testroot/a.txt")
	if !ok || a != permission.Public {
		t.Fatalf("store ok=%v a=%v", ok, a)
	}
}

func TestPutAclPrivate(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a.txt?acl"),
		strings.NewReader(`{"access":"private"}`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, _, _ := env.permstore.Get("/testroot/a.txt")
	if a != permission.Private {
		t.Fatalf("got %v", a)
	}
}

func TestPutAclInvalid(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a.txt?acl"),
		strings.NewReader(`{"access":"weird"}`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPutAclBadJSON(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a.txt?acl"),
		strings.NewReader(`not json`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPutAclTrailingTokens(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a.txt?acl"),
		strings.NewReader(`{"access":"public"}{"x":1}`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d want 400", resp.StatusCode)
	}
}

func TestDeleteAclSuccess(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := env.permstore.Put("/testroot/a.txt", permission.Public); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "DELETE", urlOf(env, "/a.txt?acl"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	resp = doReq(t, "GET", urlOf(env, "/a.txt?acl"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("followup status %d", resp.StatusCode)
	}
}

func TestDeleteAclNotFound(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "DELETE", urlOf(env, "/a.txt?acl"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// ---- AT-N53L query strictness ----

func TestAclQueryStrict(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "GET", urlOf(env, "/a.txt?acl"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("?acl status %d", resp.StatusCode)
	}
	resp = doReq(t, "GET", urlOf(env, "/a.txt?acl=x"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("?acl=x status %d", resp.StatusCode)
	}
	resp = doReq(t, "GET", urlOf(env, "/a.txt?acl&foo=1"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("?acl&foo=1 status %d", resp.StatusCode)
	}
	resp = doReq(t, "GET", urlOf(env, "/a.txt?foo=1"), nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("?foo=1 status %d", resp.StatusCode)
	}
}

// ---- AT-HFBD ----

func TestResponseNoHostACL(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", urlOf(env, "/a.txt?acl"), nil, authHdr(env.apiKey))
	defer resp.Body.Close()
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	p := body["path"]
	if !strings.HasPrefix(p, "/") {
		t.Fatalf("path %q", p)
	}
	if strings.Contains(p, "http://") || strings.Contains(p, "https://") {
		t.Fatalf("path leaks host: %q", p)
	}
}

// ---- AT-40IJ ----

func TestAuth401CoverageACL(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	cases := []struct{ method string }{
		{http.MethodGet}, {http.MethodPut}, {http.MethodDelete},
	}
	for _, c := range cases {
		resp := doReq(t, c.method, urlOf(env, "/a.txt?acl"), strings.NewReader(`{"access":"public"}`), nil)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("%s status %d want 401", c.method, resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("%s missing WWW-Authenticate", c.method)
		}
	}
}

// ACL key for directory path uses no trailing slash.
func TestAclKeyShape(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	// GET /testroot/ → urlKey should be "/testroot" (no trailing slash).
	if err := env.permstore.Put("/testroot", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", env.ts.URL+"/testroot?acl", nil, authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["path"] != "/testroot" {
		t.Fatalf("path = %q want %q", body["path"], "/testroot")
	}
}

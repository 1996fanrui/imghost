package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/1996fanrui/imghost/internal/permission"
)

func TestGetAclExplicit(t *testing.T) {
	env := testServer(t)
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", env.ts.URL+"/a.txt?acl", nil, authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["path"] != "/a.txt" || body["access"] != "private" {
		t.Fatalf("body %+v", body)
	}
}

func TestGetAclNoRule(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "GET", env.ts.URL+"/a.txt?acl", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestGetAclNoInheritance(t *testing.T) {
	env := testServer(t)
	if err := env.permstore.Put("/dir", permission.Private); err != nil {
		t.Fatal(err)
	}
	// /dir/child.txt has no own rule.
	resp := doReq(t, "GET", env.ts.URL+"/dir/child.txt?acl", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d want 404 (no inheritance)", resp.StatusCode)
	}
}

func TestPutAclPublic(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt?acl",
		strings.NewReader(`{"access":"public"}`), authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if len(b) != 0 {
		t.Fatalf("body not empty: %q", b)
	}
	a, ok, _ := env.permstore.Get("/a.txt")
	if !ok || a != permission.Public {
		t.Fatalf("store ok=%v a=%v", ok, a)
	}
}

func TestPutAclPrivate(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt?acl",
		strings.NewReader(`{"access":"private"}`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, _, _ := env.permstore.Get("/a.txt")
	if a != permission.Private {
		t.Fatalf("got %v", a)
	}
}

func TestPutAclInvalid(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt?acl",
		strings.NewReader(`{"access":"weird"}`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPutAclBadJSON(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt?acl",
		strings.NewReader(`not json`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPutAclTrailingTokens(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt?acl",
		strings.NewReader(`{"access":"public"}{"x":1}`), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d want 400", resp.StatusCode)
	}
}

func TestDeleteAclSuccess(t *testing.T) {
	env := testServer(t)
	if err := env.permstore.Put("/a.txt", permission.Public); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "DELETE", env.ts.URL+"/a.txt?acl", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	// follow-up GET → 404
	resp = doReq(t, "GET", env.ts.URL+"/a.txt?acl", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("followup status %d", resp.StatusCode)
	}
}

func TestDeleteAclNotFound(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "DELETE", env.ts.URL+"/a.txt?acl", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// ---- AT-N53L ----

func TestAclQueryStrict(t *testing.T) {
	env := testServer(t)
	// 1) ?acl → ACL handler; no rule yet so 404.
	resp := doReq(t, "GET", env.ts.URL+"/a.txt?acl", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("?acl status %d", resp.StatusCode)
	}
	// 2) ?acl=x → 400.
	resp = doReq(t, "GET", env.ts.URL+"/a.txt?acl=x", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("?acl=x status %d", resp.StatusCode)
	}
	// 3) ?acl&foo=1 → 400.
	resp = doReq(t, "GET", env.ts.URL+"/a.txt?acl&foo=1", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("?acl&foo=1 status %d", resp.StatusCode)
	}
	// 4) ?foo=1 → file handler; file doesn't exist → 404.
	resp = doReq(t, "GET", env.ts.URL+"/a.txt?foo=1", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("?foo=1 status %d", resp.StatusCode)
	}
}

// ---- AT-HFBD ----

func TestResponseNoHostACL(t *testing.T) {
	env := testServer(t)
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", env.ts.URL+"/a.txt?acl", nil, authHdr(env.apiKey))
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
	env := testServer(t)
	cases := []struct{ method string }{
		{http.MethodGet}, {http.MethodPut}, {http.MethodDelete},
	}
	for _, c := range cases {
		resp := doReq(t, c.method, env.ts.URL+"/a.txt?acl", strings.NewReader(`{"access":"public"}`), nil)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("%s status %d want 401", c.method, resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("%s missing WWW-Authenticate", c.method)
		}
	}
}

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testKey = "secret-key"

func newTestRequest(auth string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	return r
}

func assertUnauthorized(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d want 401", rr.Code)
	}
	if got := rr.Header().Get("WWW-Authenticate"); got != `Bearer realm="filehub"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestCheckAuth(t *testing.T) {
	cases := []struct {
		auth string
		want bool
	}{
		{"Bearer " + testKey, true},
		{"Bearer wrong", false},
		{"", false},
		{"Bearer ", false},
		{"Basic xyz", false},
	}
	for _, c := range cases {
		if got := CheckAuth(newTestRequest(c.auth), testKey); got != c.want {
			t.Errorf("CheckAuth(%q) = %v want %v", c.auth, got, c.want)
		}
	}
}

func TestWriteUnauthorizedHeaderBeforeBody(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteUnauthorized(rr)
	assertUnauthorized(t, rr)
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("empty body")
	}
}

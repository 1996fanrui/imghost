package server

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/1996fanrui/filehub/internal/storage"
)

// TestSwaggerUIAccessible covers AT-N942: /swagger/index.html serves the
// Swagger UI HTML (200) and does not 404 or 405.
func TestSwaggerUIAccessible(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})

	resp, err := http.Get(env.ts.URL + "/swagger/index.html")
	if err != nil {
		t.Fatalf("get swagger index: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("swagger index status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "swagger") {
		t.Fatalf("swagger index body does not mention swagger: %q", string(body))
	}
}

// TestSwaggerDocJSON asserts /swagger/doc.json is served.
func TestSwaggerDocJSON(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})

	resp, err := http.Get(env.ts.URL + "/swagger/doc.json")
	if err != nil {
		t.Fatalf("get doc.json: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("doc.json status = %d, want 200", resp.StatusCode)
	}
}

// TestSwaggerMethodNotAllowed asserts non-GET methods on reserved swagger
// routes return 405 with Allow: GET (RFC 9110 §15.5.6).
func TestSwaggerMethodNotAllowed(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})

	for _, path := range []string{"/swagger", "/swagger/index.html"} {
		req, err := http.NewRequest(http.MethodPost, env.ts.URL+path, strings.NewReader(""))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want 405", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Allow"); got != http.MethodGet {
			t.Errorf("POST %s Allow = %q, want GET", path, got)
		}
	}
}

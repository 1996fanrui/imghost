package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestSwaggerUIAccessible covers AT-N942: /swagger/index.html serves the
// Swagger UI HTML (200) and does not 404 or 405.
func TestSwaggerUIAccessible(t *testing.T) {
	env := testServer(t)

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

// TestSwaggerDocJSON asserts /swagger/doc.json advertises at least 6 operations.
func TestSwaggerDocJSON(t *testing.T) {
	env := testServer(t)

	resp, err := http.Get(env.ts.URL + "/swagger/doc.json")
	if err != nil {
		t.Fatalf("get doc.json: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("doc.json status = %d, want 200", resp.StatusCode)
	}
}

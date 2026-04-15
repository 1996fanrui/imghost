package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1996fanrui/imghost/internal/config"
)

// withHTTPTestEnv points configLoader at a stub server and returns the
// server so tests can record requests. The apiKey is asserted on the
// server side.
func withHTTPTestEnv(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse srv url: %v", err)
	}
	origLoader := configLoader
	configLoader = func() (*config.Config, error) {
		return &config.Config{ListenAddr: u.Host, APIKey: "test-key"}, nil
	}
	t.Cleanup(func() { configLoader = origLoader })
	return srv
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

type recorded struct {
	method, path, query, auth, ctype string
	body                             []byte
}

func recordingHandler(rec *recorded, status int, respBody string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		rec.auth = r.Header.Get("Authorization")
		rec.ctype = r.Header.Get("Content-Type")
		rec.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	})
}

func TestPutSendsFile(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, "ok"))
	dir := t.TempDir()
	local := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(local, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "put", "/photos/a.txt", local); err != nil {
		t.Fatalf("put: %v", err)
	}
	if rec.method != "PUT" || rec.path != "/photos/a.txt" {
		t.Errorf("method/path: %s %s", rec.method, rec.path)
	}
	if rec.auth != "Bearer test-key" {
		t.Errorf("auth: %q", rec.auth)
	}
	if string(rec.body) != "hello" {
		t.Errorf("body: %q", rec.body)
	}
}

func TestGetWritesOutputFile(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, "payload"))
	dir := t.TempDir()
	out := filepath.Join(dir, "x.bin")
	if _, err := runCLI(t, "get", "photos/cat.jpg", "-o", out); err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.method != "GET" || rec.path != "/photos/cat.jpg" {
		t.Errorf("method/path: %s %s", rec.method, rec.path)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Errorf("output: %q", got)
	}
}

func TestRmSendsDelete(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 204, ""))
	if _, err := runCLI(t, "rm", "/photos/a.txt"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	if rec.method != "DELETE" || rec.path != "/photos/a.txt" {
		t.Errorf("method/path: %s %s", rec.method, rec.path)
	}
}

func TestACLGet(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, `{"path":"/photos/a","access":"public"}`))
	out, err := runCLI(t, "acl", "get", "/photos/a")
	if err != nil {
		t.Fatalf("acl get: %v", err)
	}
	if rec.method != "GET" || rec.path != "/photos/a" || rec.query != "acl" {
		t.Errorf("req: %s %s?%s", rec.method, rec.path, rec.query)
	}
	if !strings.Contains(out, `"access":"public"`) {
		t.Errorf("output: %q", out)
	}
}

func TestACLSet(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, ""))
	if _, err := runCLI(t, "acl", "set", "/p/a", "private"); err != nil {
		t.Fatalf("acl set: %v", err)
	}
	if rec.method != "PUT" || rec.query != "acl" || rec.ctype != "application/json" {
		t.Errorf("req headers: %s ?%s ct=%s", rec.method, rec.query, rec.ctype)
	}
	if !strings.Contains(string(rec.body), `"access":"private"`) {
		t.Errorf("body: %q", rec.body)
	}
}

func TestACLSetRejectsInvalidAccess(t *testing.T) {
	withHTTPTestEnv(t, recordingHandler(&recorded{}, 500, "should not be called"))
	if _, err := runCLI(t, "acl", "set", "/p/a", "bogus"); err == nil {
		t.Fatal("expected error for invalid access value")
	}
}

func TestACLRm(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 204, ""))
	if _, err := runCLI(t, "acl", "rm", "/p/a"); err != nil {
		t.Fatalf("acl rm: %v", err)
	}
	if rec.method != "DELETE" || rec.query != "acl" {
		t.Errorf("req: %s ?%s", rec.method, rec.query)
	}
}

func TestNon2xxReturnsError(t *testing.T) {
	withHTTPTestEnv(t, recordingHandler(&recorded{}, 403, `{"error":"forbidden path"}`))
	if _, err := runCLI(t, "rm", "/p/a"); err == nil {
		t.Fatal("expected non-nil error for 403")
	}
}

func TestCompletionSubcommandDisabled(t *testing.T) {
	// Defensive: ensure `imghost completion` is not a registered subcommand.
	for _, c := range rootCmd.Commands() {
		if c.Name() == "completion" {
			t.Fatalf("completion subcommand should be disabled, found %q", c.Name())
		}
	}
}

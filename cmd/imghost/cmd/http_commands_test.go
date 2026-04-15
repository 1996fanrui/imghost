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
	origCached := cachedConfig
	configLoader = func() (*config.Config, error) {
		return &config.Config{ListenAddr: u.Host, APIKey: "test-key"}, nil
	}
	t.Cleanup(func() {
		configLoader = origLoader
		cachedConfig = origCached
	})
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

func TestPutPrintsSuccessLine(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, "ok"))
	dir := t.TempDir()
	local := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(local, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "put", "/photos/a.txt", local)
	if err != nil {
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
	if !strings.Contains(out, "uploaded /photos/a.txt") {
		t.Errorf("success line missing: %q", out)
	}
}

func TestGetDefaultSavesBasename(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, "payload"))
	dir := t.TempDir()
	t.Chdir(dir)
	out, err := runCLI(t, "get", "/photos/cat.jpg")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "cat.jpg"))
	if err != nil {
		t.Fatalf("expected ./cat.jpg to exist: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("payload: %q", got)
	}
	if !strings.Contains(out, "saved /photos/cat.jpg -> ./cat.jpg") {
		t.Errorf("success line missing: %q", out)
	}
}

func TestGetOutputFlag(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, "payload"))
	dir := t.TempDir()
	target := filepath.Join(dir, "x.bin")
	out, err := runCLI(t, "get", "/photos/cat.jpg", "-o", target)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Errorf("payload: %q", got)
	}
	if !strings.Contains(out, target) {
		t.Errorf("success line should reference custom path: %q", out)
	}
}

func TestGetStdoutMode(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, "payload"))
	out, err := runCLI(t, "get", "/photos/cat.jpg", "-o", "-")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(out, "payload") {
		t.Errorf("stdout should contain raw bytes: %q", out)
	}
	if strings.Contains(out, "saved") {
		t.Errorf("stdout mode must stay silent on success line: %q", out)
	}
}

func TestRmPrintsRemovedLine(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 204, ""))
	out, err := runCLI(t, "rm", "/photos/a.txt")
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	if rec.method != "DELETE" || rec.path != "/photos/a.txt" {
		t.Errorf("method/path: %s %s", rec.method, rec.path)
	}
	if !strings.Contains(out, "removed /photos/a.txt") {
		t.Errorf("success line missing: %q", out)
	}
}

func TestACLGetPrintsOneLiner(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, `{"path":"/photos/a","access":"public"}`))
	out, err := runCLI(t, "acl", "get", "/photos/a")
	if err != nil {
		t.Fatalf("acl get: %v", err)
	}
	if rec.method != "GET" || rec.path != "/photos/a" || rec.query != "acl" {
		t.Errorf("req: %s %s?%s", rec.method, rec.path, rec.query)
	}
	if !strings.Contains(out, "/photos/a: public") {
		t.Errorf("output: %q", out)
	}
}

func TestACLSet(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 200, ""))
	out, err := runCLI(t, "acl", "set", "/p/a", "private")
	if err != nil {
		t.Fatalf("acl set: %v", err)
	}
	if rec.method != "PUT" || rec.query != "acl" || rec.ctype != "application/json" {
		t.Errorf("req headers: %s ?%s ct=%s", rec.method, rec.query, rec.ctype)
	}
	if !strings.Contains(string(rec.body), `"access":"private"`) {
		t.Errorf("body: %q", rec.body)
	}
	if !strings.Contains(out, "acl set /p/a = private") {
		t.Errorf("output: %q", out)
	}
}

func TestACLSetRejectsInvalidAccess(t *testing.T) {
	withHTTPTestEnv(t, recordingHandler(&recorded{}, 500, "should not be called"))
	_, err := runCLI(t, "acl", "set", "/p/a", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid access value")
	}
	if !strings.Contains(err.Error(), "imghost: acl set /p/a") {
		t.Errorf("error must use unified format: %v", err)
	}
}

func TestACLRm(t *testing.T) {
	var rec recorded
	withHTTPTestEnv(t, recordingHandler(&rec, 204, ""))
	out, err := runCLI(t, "acl", "rm", "/p/a")
	if err != nil {
		t.Fatalf("acl rm: %v", err)
	}
	if rec.method != "DELETE" || rec.query != "acl" {
		t.Errorf("req: %s ?%s", rec.method, rec.query)
	}
	if !strings.Contains(out, "acl cleared /p/a") {
		t.Errorf("output: %q", out)
	}
}

func TestErrorIncludesServerMessage(t *testing.T) {
	// 404 with apierror.Response JSON body must surface the message field,
	// wrapped in the unified "imghost: <op> <target>: <reason>" form.
	withHTTPTestEnv(t, recordingHandler(&recorded{}, 404, `{"error":"not_found","message":"not found"}`))
	_, err := runCLI(t, "rm", "/p/a")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	got := err.Error()
	if !strings.Contains(got, "imghost: rm /p/a:") || !strings.Contains(got, "not found") {
		t.Errorf("unexpected error format: %q", got)
	}
	if strings.Contains(got, "404") || strings.Contains(got, "{") {
		t.Errorf("raw HTTP status / JSON leaked into user-facing error: %q", got)
	}
}

func TestCompletionSubcommandDisabled(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "completion" {
			t.Fatalf("completion subcommand should be disabled, found %q", c.Name())
		}
	}
}

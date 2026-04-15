package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/storage"
)

func doReq(t *testing.T, method, url string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new req: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func authHdr(key string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + key}
}

// urlOf returns the test server URL for a path under the single test root.
func urlOf(env *testEnv, suffix string) string {
	return env.ts.URL + "/" + testRootName + suffix
}

func TestGetFilePublic(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := os.WriteFile(filepath.Join(env.rootPath, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", urlOf(env, "/a.txt"), nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "hello" {
		t.Fatalf("body %q", string(b))
	}
}

func TestGetFilePrivateRequiresAuth(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := os.WriteFile(filepath.Join(env.rootPath, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", urlOf(env, "/a.txt"), nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("no-auth status %d", resp.StatusCode)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Fatalf("missing WWW-Authenticate")
	}
	resp = doReq(t, "GET", urlOf(env, "/a.txt"), nil, authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("auth status %d", resp.StatusCode)
	}
}

func TestGetFileNotFound(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "GET", urlOf(env, "/missing.txt"), nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestGetFileDirectoryForbidden(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := os.MkdirAll(filepath.Join(env.rootPath, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", urlOf(env, "/sub"), nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestGetUnknownRoot(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "GET", env.ts.URL+"/unknownroot/a.txt", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d want 404", resp.StatusCode)
	}
}

func TestTraversalReturns403(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "GET", env.ts.URL+"/"+testRootName+"/a/..%2fb", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("status %d want 403", resp.StatusCode)
	}
}

func TestPutFileOverwrite(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	path := filepath.Join(env.rootPath, "a.txt")
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("NEW"), authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "NEW" {
		t.Fatalf("disk %q", string(b))
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["path"] != "/testroot/a.txt" {
		t.Fatalf("resp path %q", body["path"])
	}
}

func TestPutFileAutoMkdir(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a/b/c.txt"), strings.NewReader("X"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, err := os.ReadFile(filepath.Join(env.rootPath, "a/b/c.txt"))
	if err != nil || string(b) != "X" {
		t.Fatalf("disk err=%v body=%q", err, b)
	}
}

func TestPutFileWithXAccess(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	h := authHdr(env.apiKey)
	h["X-Access"] = "private"
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("X"), h)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, ok, _ := env.permstore.Get("/testroot/a.txt")
	if !ok || a != permission.Private {
		t.Fatalf("permstore ok=%v a=%v", ok, a)
	}
}

func TestPutFileNoXAccessNoRule(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("X"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	_, ok, _ := env.permstore.Get("/testroot/a.txt")
	if ok {
		t.Fatalf("permstore should not have rule")
	}
}

func TestPutFileNoXAccessPreservesExistingRule(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("X"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, ok, _ := env.permstore.Get("/testroot/a.txt")
	if !ok || a != permission.Private {
		t.Fatalf("rule lost: ok=%v a=%v", ok, a)
	}
}

func TestPutFileXAccessOverrides(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("X"), h)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, _, _ := env.permstore.Get("/testroot/a.txt")
	if a != permission.Public {
		t.Fatalf("want public got %v", a)
	}
}

func TestPutFileInvalidXAccess(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	h := authHdr(env.apiKey)
	h["X-Access"] = "weird"
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("X"), h)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// TestPutStreaming implements AT-IVQK: verify the handler writes bytes to
// the temp file before the request body reaches EOF.
func TestPutStreaming(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	chunk := bytes.Repeat([]byte("A"), 64*1024)
	totalChunks := 4
	reader := newBlockingReader(chunk, totalChunks)

	targetDir := env.rootPath

	done := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp := doReq(t, "PUT", urlOf(env, "/big.bin"), reader, authHdr(env.apiKey))
		done <- resp
	}()

	reader.release()

	deadline := time.Now().Add(3 * time.Second)
	seen := false
outer:
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			errCh <- err
			break
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".tmp") {
				info, ierr := os.Stat(filepath.Join(targetDir, e.Name()))
				if ierr == nil && info.Size() >= int64(len(chunk)) {
					seen = true
					break outer
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !seen {
		t.Fatalf("temp file never reached chunk size before EOF")
	}

	for i := 1; i < totalChunks; i++ {
		reader.release()
	}
	reader.close()

	select {
	case resp := <-done:
		defer resp.Body.Close()
		if resp.StatusCode != 201 {
			t.Fatalf("status %d", resp.StatusCode)
		}
	case err := <-errCh:
		t.Fatalf("scan err: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatalf("put did not finish")
	}

	info, err := os.Stat(filepath.Join(env.rootPath, "big.bin"))
	if err != nil {
		t.Fatalf("stat final: %v", err)
	}
	if info.Size() != int64(len(chunk)*totalChunks) {
		t.Fatalf("final size %d want %d", info.Size(), len(chunk)*totalChunks)
	}
}

type blockingReader struct {
	chunk    []byte
	gate     chan struct{}
	closed   chan struct{}
	pending  []byte
	released int
	total    int
}

func newBlockingReader(chunk []byte, total int) *blockingReader {
	return &blockingReader{
		chunk:  chunk,
		gate:   make(chan struct{}, total+1),
		closed: make(chan struct{}),
		total:  total,
	}
}

func (b *blockingReader) release() { b.gate <- struct{}{} }
func (b *blockingReader) close()   { close(b.closed) }

func (b *blockingReader) Read(p []byte) (int, error) {
	if len(b.pending) > 0 {
		n := copy(p, b.pending)
		b.pending = b.pending[n:]
		return n, nil
	}
	if b.released >= b.total {
		select {
		case <-b.closed:
			return 0, io.EOF
		case <-b.gate:
			return 0, io.EOF
		case <-time.After(5 * time.Second):
			return 0, io.EOF
		}
	}
	select {
	case <-b.gate:
		b.released++
		b.pending = append([]byte(nil), b.chunk...)
		n := copy(p, b.pending)
		b.pending = b.pending[n:]
		return n, nil
	default:
	}
	select {
	case <-b.gate:
		b.released++
		b.pending = append([]byte(nil), b.chunk...)
		n := copy(p, b.pending)
		b.pending = b.pending[n:]
		return n, nil
	case <-b.closed:
		return 0, io.EOF
	case <-time.After(5 * time.Second):
		return 0, io.EOF
	}
}

// ---- AT-W81S Atomicity ----

type fakeFS struct {
	storage.FS
	renameFail bool
}

func (f *fakeFS) AtomicWrite(abs string, r io.Reader) error {
	if f.renameFail {
		_, _ = io.Copy(io.Discard, r)
		return errors.New("injected rename failure")
	}
	return f.FS.AtomicWrite(abs, r)
}

func TestPutAtomicityRenameFailureOverwrite(t *testing.T) {
	ff := &fakeFS{FS: storage.OSFS{}, renameFail: true}
	env := newTestServer(t, ff)
	path := filepath.Join(env.rootPath, "a.txt")
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("NEW"), h)
	resp.Body.Close()
	if resp.StatusCode < 500 {
		t.Fatalf("want 5xx got %d", resp.StatusCode)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "OLD" {
		t.Fatalf("disk %q", string(b))
	}
	a, ok, _ := env.permstore.Get("/testroot/a.txt")
	if !ok || a != permission.Private {
		t.Fatalf("rollback failed ok=%v a=%v", ok, a)
	}
}

func TestPutAtomicityPermstoreFailure(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	path := filepath.Join(env.rootPath, "a.txt")
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	_ = env.permstore.Close()

	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("NEW"), h)
	resp.Body.Close()
	if resp.StatusCode < 500 {
		t.Fatalf("want 5xx got %d", resp.StatusCode)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "OLD" {
		t.Fatalf("disk %q", string(b))
	}
	entries, _ := os.ReadDir(env.rootPath)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("tmp leaked: %s", e.Name())
		}
	}
}

func TestPutAtomicityNewFileRenameFailure(t *testing.T) {
	ff := &fakeFS{FS: storage.OSFS{}, renameFail: true}
	env := newTestServer(t, ff)
	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", urlOf(env, "/new.txt"), strings.NewReader("NEW"), h)
	resp.Body.Close()
	if resp.StatusCode < 500 {
		t.Fatalf("want 5xx got %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(env.rootPath, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("file should not exist: %v", err)
	}
	_, ok, _ := env.permstore.Get("/testroot/new.txt")
	if ok {
		t.Fatalf("permstore should have been rolled back")
	}
}

func TestDeleteFileSuccess(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	path := filepath.Join(env.rootPath, "a.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "DELETE", urlOf(env, "/a.txt"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists")
	}
}

func TestDeleteFileNotFound(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "DELETE", urlOf(env, "/missing.txt"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestDeleteFileDirectory(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := os.MkdirAll(filepath.Join(env.rootPath, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "DELETE", urlOf(env, "/subdir"), nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("status %d want 403", resp.StatusCode)
	}
}

// ---- reserved path behavior ----

func TestReservedSwaggerWriteRejected(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	cases := []struct {
		method string
		path   string
		want   int
	}{
		{"PUT", "/swagger/foo", 405},
		{"DELETE", "/swagger/foo", 405},
		{"PUT", "/swagger", 405},
		{"PUT", "/swagger/index.html", 405},
		{"DELETE", "/swagger", 405},
	}
	for _, c := range cases {
		resp := doReq(t, c.method, env.ts.URL+c.path, strings.NewReader("x"), authHdr(env.apiKey))
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s %s = %d want %d", c.method, c.path, resp.StatusCode, c.want)
		}
	}
}

func TestReservedSwaggerACL(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", env.ts.URL+"/swagger/foo?acl", strings.NewReader("{}"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatalf("status %d want 405", resp.StatusCode)
	}
}

// ---- AT-77K6 root mapping ----

func TestRootMapping(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := os.MkdirAll(filepath.Join(env.rootPath, "photos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.rootPath, "photos/x.jpg"), []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp := doReq(t, "GET", urlOf(env, "/photos/x.jpg"), nil, nil)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "IMG" {
		t.Fatalf("photos body %q", b)
	}

	// Verify ResolvePath still maps suffix directly under rootPath.
	_, phys, err := ResolvePath(env.rootPath, "/photos/x.jpg")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if phys != filepath.Join(env.rootPath, "photos/x.jpg") {
		t.Fatalf("physical mismatch: %s", phys)
	}
}

// ---- AT-HFBD response must not leak host ----

func TestResponseNoHost(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	resp := doReq(t, "PUT", urlOf(env, "/a.txt"), strings.NewReader("X"), authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	p := body["path"]
	if !strings.HasPrefix(p, "/") {
		t.Fatalf("path not rooted: %q", p)
	}
	if strings.Contains(p, "http://") || strings.Contains(p, "https://") {
		t.Fatalf("path leaks host: %q", p)
	}
}

// ---- AT-40IJ 401 coverage ----

func TestAuth401Coverage(t *testing.T) {
	env := newTestServer(t, storage.OSFS{})
	if err := os.WriteFile(filepath.Join(env.rootPath, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/testroot/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		method, path string
	}{
		{"PUT", "/b.txt"},
		{"DELETE", "/a.txt"},
		{"GET", "/a.txt"},
	}
	for _, c := range cases {
		resp := doReq(t, c.method, urlOf(env, c.path), strings.NewReader("x"), nil)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("%s %s status %d want 401", c.method, c.path, resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("%s %s missing WWW-Authenticate", c.method, c.path)
		}
	}
}

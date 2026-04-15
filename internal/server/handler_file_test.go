package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
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

func TestGetFilePublic(t *testing.T) {
	env := testServer(t)
	if err := os.WriteFile(filepath.Join(env.dataDir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", env.ts.URL+"/a.txt", nil, nil)
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
	env := testServer(t)
	if err := os.WriteFile(filepath.Join(env.dataDir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	// no auth → 401
	resp := doReq(t, "GET", env.ts.URL+"/a.txt", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("no-auth status %d", resp.StatusCode)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Fatalf("missing WWW-Authenticate")
	}
	// with auth → 200
	resp = doReq(t, "GET", env.ts.URL+"/a.txt", nil, authHdr(env.apiKey))
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("auth status %d", resp.StatusCode)
	}
}

func TestGetFileNotFound(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "GET", env.ts.URL+"/missing.txt", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestGetFileDirectoryForbidden(t *testing.T) {
	env := testServer(t)
	if err := os.MkdirAll(filepath.Join(env.dataDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "GET", env.ts.URL+"/sub", nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestPutFileOverwrite(t *testing.T) {
	env := testServer(t)
	path := filepath.Join(env.dataDir, "a.txt")
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("NEW"), authHdr(env.apiKey))
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
	if body["path"] != "/a.txt" {
		t.Fatalf("resp path %q", body["path"])
	}
}

func TestPutFileAutoMkdir(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a/b/c.txt", strings.NewReader("X"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	b, err := os.ReadFile(filepath.Join(env.dataDir, "a/b/c.txt"))
	if err != nil || string(b) != "X" {
		t.Fatalf("disk err=%v body=%q", err, b)
	}
}

func TestPutFileWithXAccess(t *testing.T) {
	env := testServer(t)
	h := authHdr(env.apiKey)
	h["X-Access"] = "private"
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("X"), h)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, ok, _ := env.permstore.Get("/a.txt")
	if !ok || a != permission.Private {
		t.Fatalf("permstore ok=%v a=%v", ok, a)
	}
}

func TestPutFileNoXAccessNoRule(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("X"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	_, ok, _ := env.permstore.Get("/a.txt")
	if ok {
		t.Fatalf("permstore should not have rule")
	}
}

func TestPutFileNoXAccessPreservesExistingRule(t *testing.T) {
	env := testServer(t)
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("X"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, ok, _ := env.permstore.Get("/a.txt")
	if !ok || a != permission.Private {
		t.Fatalf("rule lost: ok=%v a=%v", ok, a)
	}
}

func TestPutFileXAccessOverrides(t *testing.T) {
	env := testServer(t)
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("X"), h)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	a, _, _ := env.permstore.Get("/a.txt")
	if a != permission.Public {
		t.Fatalf("want public got %v", a)
	}
}

func TestPutFileInvalidXAccess(t *testing.T) {
	env := testServer(t)
	h := authHdr(env.apiKey)
	h["X-Access"] = "weird"
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("X"), h)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// TestPutStreaming implements AT-IVQK: verify the handler writes bytes to
// the temp file before the request body reaches EOF.
func TestPutStreaming(t *testing.T) {
	env := testServer(t)
	chunk := bytes.Repeat([]byte("A"), 64*1024) // 64 KiB
	totalChunks := 4
	reader := newBlockingReader(chunk, totalChunks)

	targetDir := env.dataDir // file at "/big.bin" lives under dataDir

	done := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp := doReq(t, "PUT", env.ts.URL+"/big.bin", reader, authHdr(env.apiKey))
		done <- resp
	}()

	// Allow reader to serve the first chunk.
	reader.release()

	// Wait until a *.tmp appears with at least chunk-size bytes.
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

	// Release remaining chunks.
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

	info, err := os.Stat(filepath.Join(env.dataDir, "big.bin"))
	if err != nil {
		t.Fatalf("stat final: %v", err)
	}
	if info.Size() != int64(len(chunk)*totalChunks) {
		t.Fatalf("final size %d want %d", info.Size(), len(chunk)*totalChunks)
	}
}

// blockingReader serves exactly one chunk per release() call, blocking until
// each release. close() signals EOF after all pending releases consumed.
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
			// extra release treated as no-op → EOF.
			return 0, io.EOF
		case <-time.After(5 * time.Second):
			return 0, io.EOF
		}
	}
	// Drain any buffered gate signals non-blockingly before falling back to
	// the select that also watches closed/timeout; this prevents races where
	// `closed` wins over still-buffered `gate` sends.
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
		// Drain body to simulate partial work, then fail (temp is cleaned
		// internally in real impl; here no temp is left).
		_, _ = io.Copy(io.Discard, r)
		return errors.New("injected rename failure")
	}
	return f.FS.AtomicWrite(abs, r)
}

func TestPutAtomicityRenameFailureOverwrite(t *testing.T) {
	ff := &fakeFS{FS: storage.OSFS{}, renameFail: true}
	env := newTestEnvWithFS(t, ff, permission.Public)
	path := filepath.Join(env.dataDir, "a.txt")
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("NEW"), h)
	resp.Body.Close()
	if resp.StatusCode < 500 {
		t.Fatalf("want 5xx got %d", resp.StatusCode)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "OLD" {
		t.Fatalf("disk %q", string(b))
	}
	a, ok, _ := env.permstore.Get("/a.txt")
	if !ok || a != permission.Private {
		t.Fatalf("rollback failed ok=%v a=%v", ok, a)
	}
}

// failingPermFS simulates permstore write failure by wrapping normal FS;
// actual permstore failure requires injecting into storage, not FS.
// For AT-W81S's permstore-Put-failure case we use a closed permstore.
func TestPutAtomicityPermstoreFailure(t *testing.T) {
	env := testServer(t)
	path := filepath.Join(env.dataDir, "a.txt")
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
		t.Fatal(err)
	}
	// Close permstore so all subsequent writes fail.
	_ = env.permstore.Close()

	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("NEW"), h)
	resp.Body.Close()
	if resp.StatusCode < 500 {
		t.Fatalf("want 5xx got %d", resp.StatusCode)
	}
	// File must remain OLD.
	b, _ := os.ReadFile(path)
	if string(b) != "OLD" {
		t.Fatalf("disk %q", string(b))
	}
	// No temp file should remain in dataDir.
	entries, _ := os.ReadDir(env.dataDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("tmp leaked: %s", e.Name())
		}
	}
}

func TestPutAtomicityNewFileRenameFailure(t *testing.T) {
	ff := &fakeFS{FS: storage.OSFS{}, renameFail: true}
	env := newTestEnvWithFS(t, ff, permission.Public)
	h := authHdr(env.apiKey)
	h["X-Access"] = "public"
	resp := doReq(t, "PUT", env.ts.URL+"/new.txt", strings.NewReader("NEW"), h)
	resp.Body.Close()
	if resp.StatusCode < 500 {
		t.Fatalf("want 5xx got %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(env.dataDir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("file should not exist: %v", err)
	}
	_, ok, _ := env.permstore.Get("/new.txt")
	if ok {
		t.Fatalf("permstore should have been rolled back")
	}
}

func TestDeleteFileSuccess(t *testing.T) {
	env := testServer(t)
	path := filepath.Join(env.dataDir, "a.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "DELETE", env.ts.URL+"/a.txt", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists")
	}
}

func TestDeleteFileNotFound(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "DELETE", env.ts.URL+"/missing.txt", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestDeleteFileDirectory(t *testing.T) {
	env := testServer(t)
	if err := os.MkdirAll(filepath.Join(env.dataDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	resp := doReq(t, "DELETE", env.ts.URL+"/subdir", nil, authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("status %d want 403", resp.StatusCode)
	}
}

// ---- AT-K15B reserved path ----

func TestReservedWriteRejected(t *testing.T) {
	env := testServer(t)
	cases := []struct {
		method string
		path   string
		want   int
	}{
		{"PUT", "/swagger/foo", 405},         // goes to swagger handler first
		{"DELETE", "/swagger/foo", 405},      // ditto
		{"PUT", "/swagger", 405},             // swagger handler
		{"PUT", "/swagger/", 405},            // swagger handler (prefix)
		{"PUT", "/swagger/index.html", 405},  // swagger handler
		{"DELETE", "/swagger", 405},          // swagger handler
	}
	for _, c := range cases {
		resp := doReq(t, c.method, env.ts.URL+c.path, strings.NewReader("x"), authHdr(env.apiKey))
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s %s = %d want %d", c.method, c.path, resp.StatusCode, c.want)
		}
	}
}

// TestReservedSwaggerACL covers AT-K15B "PUT /swagger/foo?acl → 405".
func TestReservedSwaggerACL(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/swagger/foo?acl", strings.NewReader("{}"), authHdr(env.apiKey))
	resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatalf("status %d want 405", resp.StatusCode)
	}
}

// TestReservedFileHandlerRejection verifies IsReserved is enforced by the
// file handler even when someone reaches it directly (e.g. if router changes).
func TestReservedFileHandlerDirect(t *testing.T) {
	env := testServer(t)
	h := &FileHandler{
		DataDir:   env.dataDir,
		FS:        storage.OSFS{},
		PermStore: env.permstore,
		Resolver:  &permission.Resolver{Store: env.permstore, Default: permission.Public},
		APIKey:    env.apiKey,
	}
	req, _ := http.NewRequest("PUT", "/swagger/bad", strings.NewReader("x"))
	req.Header.Set("Authorization", "Bearer "+env.apiKey)
	rw := &recordingRW{header: http.Header{}}
	h.ServeHTTP(rw, req)
	if rw.status != 400 {
		t.Fatalf("status %d want 400", rw.status)
	}
}

type recordingRW struct {
	header http.Header
	buf    bytes.Buffer
	status int
}

func (r *recordingRW) Header() http.Header     { return r.header }
func (r *recordingRW) Write(b []byte) (int, error) { return r.buf.Write(b) }
func (r *recordingRW) WriteHeader(s int)        { r.status = s }

// TestReservedNoStrayLiterals implements AT-K15B's grep assertion: the string
// literal "/swagger" family (bare, trailing-slash, trailing-star) only
// appears in reserved.go and the single router registration line.
func TestReservedNoStrayLiterals(t *testing.T) {
	// Broadened regex to also match the router pattern "/swagger/*".
	cmd := exec.Command("grep", "-RE", `"/swagger[/*]*"`, "internal/server", "cmd")
	cmd.Dir = repoRoot(t)
	out, _ := cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var offenders []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		// strip file:content
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		file := parts[0]
		if strings.HasSuffix(file, "reserved.go") {
			continue
		}
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		offenders = append(offenders, line)
	}
	if len(offenders) != 1 {
		t.Fatalf("expected exactly 1 non-reserved/non-test line containing /swagger literal, got %d:\n%s",
			len(offenders), strings.Join(offenders, "\n"))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "env", "GOMOD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go env: %v", err)
	}
	return filepath.Dir(strings.TrimSpace(string(out)))
}

// ---- AT-77K6 ----

func TestDataDirMapping(t *testing.T) {
	env := testServer(t)
	if err := os.MkdirAll(filepath.Join(env.dataDir, "photos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(env.dataDir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.dataDir, "photos/x.jpg"), []byte("IMG"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.dataDir, "docs/y.md"), []byte("MD"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp := doReq(t, "GET", env.ts.URL+"/photos/x.jpg", nil, nil)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "IMG" {
		t.Fatalf("photos body %q", b)
	}

	resp = doReq(t, "GET", env.ts.URL+"/docs/y.md", nil, nil)
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "MD" {
		t.Fatalf("docs body %q", b)
	}

	// Verify internal resolver maps the URL path directly under dataDir
	// without any "/data/" prefix.
	_, phys, err := ResolvePath(env.dataDir, "/photos/x.jpg")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if phys != filepath.Join(env.dataDir, "photos/x.jpg") {
		t.Fatalf("physical mismatch: %s", phys)
	}
}

// ---- AT-HFBD ----

func TestResponseNoHost(t *testing.T) {
	env := testServer(t)
	resp := doReq(t, "PUT", env.ts.URL+"/a.txt", strings.NewReader("X"), authHdr(env.apiKey))
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

// ---- AT-40IJ ----

func TestAuth401Coverage(t *testing.T) {
	env := testServer(t)
	// Create a private file for GET-private case.
	if err := os.WriteFile(filepath.Join(env.dataDir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := env.permstore.Put("/a.txt", permission.Private); err != nil {
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
		resp := doReq(t, c.method, env.ts.URL+c.path, strings.NewReader("x"), nil)
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("%s %s status %d want 401", c.method, c.path, resp.StatusCode)
		}
		if !strings.Contains(resp.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("%s %s missing WWW-Authenticate", c.method, c.path)
		}
	}
}

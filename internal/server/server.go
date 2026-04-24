package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"
	bolt "go.etcd.io/bbolt"

	"github.com/1996fanrui/filehub/internal/apierror"
	"github.com/1996fanrui/filehub/internal/config"
	"github.com/1996fanrui/filehub/internal/permission"
	"github.com/1996fanrui/filehub/internal/storage"

	// Blank import: register generated swagger doc with swag registry so
	// httpSwagger.WrapHandler can serve /swagger/doc.json.
	_ "github.com/1996fanrui/filehub/docs"
)

// New builds the HTTP handler, wiring the file/ACL handlers, health, and
// Swagger UI against the supplied config.
func New(cfg *config.Config, fs storage.FS, permstore *storage.PermStore) http.Handler {
	resolver := &permission.Resolver{Store: permstore}
	file := &FileHandler{
		FS:        fs,
		PermStore: permstore,
		Resolver:  resolver,
		APIKey:    cfg.APIKey,
	}
	acl := &ACLHandler{
		PermStore: permstore,
		APIKey:    cfg.APIKey,
	}
	return newRouter(cfg, file, acl)
}

type routeKind int

const (
	routeFile routeKind = iota
	routeACL
)

// routeByQuery inspects the request query to pick the handler. The "acl"
// subresource is a bare key with an empty value and must be the only query
// parameter; "acl=value" or "acl + other keys" returns an error (400).
// Any other queries are ignored (passed through to file handler).
func routeByQuery(req *http.Request) (routeKind, error) {
	q := req.URL.Query()
	vals, present := q["acl"]
	if !present {
		return routeFile, nil
	}
	if len(vals) != 1 || vals[0] != "" {
		return routeFile, errors.New("acl must be a bare query key without value")
	}
	if len(q) != 1 {
		return routeFile, errors.New("acl must be the only query key")
	}
	return routeACL, nil
}

func swaggerEntry() http.Handler {
	ui := httpSwagger.WrapHandler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			apierror.MethodNotAllowed(w, "method not allowed")
			return
		}
		ui(w, r)
	})
}

// boltOpenTimeout bounds how long bbolt waits for the exclusive file lock.
// Exposed as a package-level var (not const) so tests can shrink it; the
// default 5s matches REQ-BO05 fail-fast semantics for production.
var boltOpenTimeout = 5 * time.Second

// Start loads dependencies, runs the HTTP server, and handles graceful
// shutdown on SIGINT/SIGTERM. bbolt is opened with boltOpenTimeout so a
// stale lock surfaces as a startup error rather than hanging.
func Start(ctx context.Context, cfg *config.Config) error {
	permstore, err := storage.OpenWithOptions(cfg.DBPath, &bolt.Options{Timeout: boltOpenTimeout})
	if err != nil {
		return fmt.Errorf("open permstore at %s: %w", cfg.DBPath, err)
	}

	handler := New(cfg, storage.DefaultFS, permstore)
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	stopCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-stopCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		_ = permstore.Close()
		return nil
	case err := <-errCh:
		_ = permstore.Close()
		return err
	}
}

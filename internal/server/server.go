package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/1996fanrui/imghost/internal/apierror"
	"github.com/1996fanrui/imghost/internal/config"
	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/storage"

	// Blank import: register generated swagger doc with swag registry so
	// httpSwagger.WrapHandler can serve /swagger/doc.json.
	_ "github.com/1996fanrui/imghost/docs"
)

// New builds the HTTP handler, wiring the file/ACL handlers and the Swagger UI.
func New(cfg *config.Config, fs storage.FS, permstore *storage.PermStore) http.Handler {
	resolver := &permission.Resolver{Store: permstore, Default: cfg.DefaultAccess}
	file := &FileHandler{
		DataDir:   cfg.DataDir,
		FS:        fs,
		PermStore: permstore,
		Resolver:  resolver,
		APIKey:    cfg.APIKey,
	}
	acl := &ACLHandler{
		DataDir:   cfg.DataDir,
		PermStore: permstore,
		APIKey:    cfg.APIKey,
	}

	r := chi.NewRouter()

	// Register swagger UI and doc JSON. Non-GET returns 405. Derive the bare
	// path from the pattern at runtime so reserved.go remains the single
	// source of truth for swagger path literals.
	swaggerPattern := "/swagger/*"
	r.Handle(swaggerPattern, swaggerEntry())
	r.Handle(swaggerPattern[:len(swaggerPattern)-2], swaggerEntry())

	// Catch-all: dispatch between ACL and file handler based on query.
	r.HandleFunc("/*", func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodPut && req.Method != http.MethodDelete {
			apierror.MethodNotAllowed(w, "method not allowed")
			return
		}
		route, err := routeByQuery(req)
		if err != nil {
			apierror.BadRequest(w, err.Error())
			return
		}
		switch route {
		case routeACL:
			acl.ServeHTTP(w, req)
		case routeFile:
			file.ServeHTTP(w, req)
		}
	})

	return r
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
	// acl present: must be empty value AND only query key.
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

// Start loads dependencies, runs the HTTP server, and handles graceful
// shutdown on SIGINT/SIGTERM.
func Start(ctx context.Context, cfg *config.Config) error {
	permstore, err := storage.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open permstore: %w", err)
	}

	handler := New(cfg, storage.DefaultFS, permstore)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
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

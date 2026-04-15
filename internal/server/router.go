package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/1996fanrui/imghost/internal/apierror"
	"github.com/1996fanrui/imghost/internal/config"
	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/reserved"
)

// resolvedContext carries the router's per-request resolution output
// (url ACL key + physical filesystem path + root-scoped effective default
// access) to handlers. defaultAccess is pre-computed here so the permission
// resolver can honor per-root `access` overrides without re-looking up the
// matching root.
type resolvedContext struct {
	urlKey        string
	physical      string
	defaultAccess permission.Access
}

type resolvedCtxKeyType struct{}

var resolvedCtxKey = resolvedCtxKeyType{}

func withResolved(r *http.Request, rc *resolvedContext) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), resolvedCtxKey, rc))
}

func resolvedFrom(r *http.Request) *resolvedContext {
	if rc, ok := r.Context().Value(resolvedCtxKey).(*resolvedContext); ok {
		return rc
	}
	return nil
}

// newRouter wires all routes. Reserved routes are mounted on a sub-router
// registered before the catch-all so chi's per-route MethodNotAllowed fires
// with Allow: GET (RFC 9110 §15.5.6) for non-GET requests to /swagger.
// The catch-all resolves the URL first segment against cfg.Roots.
func newRouter(cfg *config.Config, file *FileHandler, acl *ACLHandler) http.Handler {
	r := chi.NewRouter()

	// Reserved swagger paths: GET is served by swaggerEntry; any other
	// method is short-circuited to 405 + Allow: GET (RFC 9110 §15.5.6)
	// before the catch-all can swallow the request. We cannot rely on
	// chi's per-router MethodNotAllowed because the catch-all `/*`
	// matches any method and runs first for non-GET requests.
	swaggerHandler := swaggerEntry()
	swaggerMethodGate := func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		swaggerHandler.ServeHTTP(w, req)
	}
	r.HandleFunc("/swagger", swaggerMethodGate)
	r.HandleFunc("/swagger/*", swaggerMethodGate)

	r.HandleFunc("/*", catchAll(cfg, file, acl))
	return r
}

// catchAll implements the root-name-dispatch routing layer.
func catchAll(cfg *config.Config, file *FileHandler, acl *ACLHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodPut && req.Method != http.MethodDelete {
			apierror.MethodNotAllowed(w, "method not allowed")
			return
		}
		route, err := routeByQuery(req)
		if err != nil {
			apierror.BadRequest(w, err.Error())
			return
		}

		escaped := req.URL.EscapedPath()
		firstSeg, rest := splitFirstSeg(escaped)
		if firstSeg == "" {
			apierror.NotFound(w, "not found")
			return
		}
		// Defensive: reserved names must never reach here; if they do
		// (e.g. "/swagger/extra" without the sub-router match, or a
		// future reserved name missing a handler), short-circuit to 404.
		if reserved.IsName(firstSeg) {
			apierror.NotFound(w, "not found")
			return
		}
		root, ok := cfg.RootByName(firstSeg)
		if !ok {
			apierror.NotFound(w, "not found")
			return
		}
		suffix := rest
		if suffix == "" {
			suffix = "/"
		}
		cleaned, physical, err := ResolvePath(root.Path, suffix)
		if err != nil {
			if errors.Is(err, ErrTraversal) || errors.Is(err, ErrJoinEscape) || errors.Is(err, ErrSymlinkEscape) {
				apierror.Forbidden(w, "forbidden path")
				return
			}
			apierror.BadRequest(w, "invalid path")
			return
		}
		urlKey := "/" + root.Name + cleaned
		if cleaned == "/" {
			urlKey = "/" + root.Name
		}
		rc := &resolvedContext{
			urlKey:        urlKey,
			physical:      physical,
			defaultAccess: root.EffectiveAccess(cfg.DefaultAccess),
		}
		next := withResolved(req, rc)

		switch route {
		case routeACL:
			acl.ServeHTTP(w, next)
		case routeFile:
			file.ServeHTTP(w, next)
		}
	}
}

// splitFirstSeg splits the leading "/first/rest..." into (first, "/rest...").
// For "/first" returns ("first", "").
func splitFirstSeg(escaped string) (string, string) {
	if !strings.HasPrefix(escaped, "/") {
		return "", ""
	}
	after := escaped[1:]
	if after == "" {
		return "", ""
	}
	if i := strings.IndexByte(after, '/'); i >= 0 {
		return after[:i], after[i:]
	}
	return after, ""
}

package server

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/1996fanrui/filehub/internal/apierror"
)

const bearerPrefix = "Bearer "

// WriteUnauthorized emits a 401 JSON body alongside the WWW-Authenticate
// challenge. All 401 responses in the server must go through this helper so
// the challenge header is never omitted (RFC 6750 §3).
func WriteUnauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="filehub"`)
	apierror.Unauthorized(w, "missing or invalid bearer token")
}

// CheckAuth reports whether r carries a valid Bearer token matching apiKey.
// Used by handlers that authenticate conditionally (e.g. GET on a file whose
// effective access is private).
func CheckAuth(r *http.Request, apiKey string) bool {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, bearerPrefix) {
		return false
	}
	token := h[len(bearerPrefix):]
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) == 1
}

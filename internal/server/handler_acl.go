package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/1996fanrui/imghost/internal/apierror"
	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/storage"
)

// ACLHandler serves GET/PUT/DELETE /<path>?acl.
// Reserved-path restrictions intentionally DO NOT apply here: the router sends
// /swagger/* to the swagger handler before this one is reached.
type ACLHandler struct {
	DataDir   string
	PermStore *storage.PermStore
	APIKey    string
}

type aclBody struct {
	Access string `json:"access"`
}

func (h *ACLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(r, h.APIKey) {
		WriteUnauthorized(w)
		return
	}
	if !validateACLQuery(r) {
		apierror.BadRequest(w, "invalid query")
		return
	}
	cleaned, _, err := ResolvePath(h.DataDir, r.URL.EscapedPath())
	if err != nil {
		if errors.Is(err, ErrSymlinkEscape) {
			apierror.Forbidden(w, "symlink escape")
			return
		}
		apierror.BadRequest(w, "invalid path")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.doGet(w, cleaned)
	case http.MethodPut:
		h.doPut(w, r, cleaned)
	case http.MethodDelete:
		h.doDelete(w, cleaned)
	default:
		apierror.MethodNotAllowed(w, "method not allowed")
	}
}

// validateACLQuery enforces: exactly one query key "acl" with an empty value.
func validateACLQuery(r *http.Request) bool {
	q := r.URL.Query()
	if len(q) != 1 {
		return false
	}
	vals, ok := q["acl"]
	if !ok {
		return false
	}
	if len(vals) != 1 || vals[0] != "" {
		return false
	}
	return true
}

// doGet returns the explicit ACL for a path.
//
// @Summary      Get ACL
// @Description  Access via /<path>?acl (bare query parameter). Documented here as /{path}/acl for OpenAPI 2.0 tooling compatibility; use the ?acl form in actual requests.
// @Tags         acl
// @Produce      application/json
// @Param        path           path    string  true  "object path (wildcard)"
// @Param        acl            query   string  true  "must be the bare key 'acl'"
// @Param        Authorization  header  string  true  "Bearer <API_KEY>"
// @Success      200  {object}  map[string]string
// @Failure      401  {object}  apierror.Response
// @Failure      404  {object}  apierror.Response
// @Router       /{path}/acl [get]
func (h *ACLHandler) doGet(w http.ResponseWriter, cleaned string) {
	a, ok, err := h.PermStore.Get(cleaned)
	if err != nil {
		apierror.InternalError(w, "permstore read")
		return
	}
	if !ok {
		apierror.NotFound(w, "no explicit rule")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"path": cleaned, "access": string(a)})
}

// doPut sets the explicit ACL for a path.
//
// @Summary      Set ACL
// @Description  Access via /<path>?acl (bare query parameter). Documented here as /{path}/acl for OpenAPI 2.0 tooling compatibility; use the ?acl form in actual requests.
// @Tags         acl
// @Accept       application/json
// @Param        path           path    string  true  "object path (wildcard)"
// @Param        acl            query   string  true  "must be the bare key 'acl'"
// @Param        Authorization  header  string  true  "Bearer <API_KEY>"
// @Param        body           body    aclBody true  "access payload"
// @Success      200
// @Failure      400  {object}  apierror.Response
// @Failure      401  {object}  apierror.Response
// @Router       /{path}/acl [put]
func (h *ACLHandler) doPut(w http.ResponseWriter, r *http.Request, cleaned string) {
	var b aclBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		apierror.BadRequest(w, "invalid json")
		return
	}
	if dec.More() {
		apierror.BadRequest(w, "request body must contain exactly one JSON object")
		return
	}
	a, err := permission.Parse(b.Access)
	if err != nil {
		apierror.BadRequest(w, "invalid access")
		return
	}
	if err := h.PermStore.Put(cleaned, a); err != nil {
		apierror.InternalError(w, "permstore write")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// doDelete removes the explicit ACL for a path.
//
// @Summary      Delete ACL
// @Description  Access via /<path>?acl (bare query parameter). Documented here as /{path}/acl for OpenAPI 2.0 tooling compatibility; use the ?acl form in actual requests.
// @Tags         acl
// @Param        path           path    string  true  "object path (wildcard)"
// @Param        acl            query   string  true  "must be the bare key 'acl'"
// @Param        Authorization  header  string  true  "Bearer <API_KEY>"
// @Success      204
// @Failure      401  {object}  apierror.Response
// @Failure      404  {object}  apierror.Response
// @Router       /{path}/acl [delete]
func (h *ACLHandler) doDelete(w http.ResponseWriter, cleaned string) {
	_, ok, err := h.PermStore.Get(cleaned)
	if err != nil {
		apierror.InternalError(w, "permstore read")
		return
	}
	if !ok {
		apierror.NotFound(w, "no explicit rule")
		return
	}
	if err := h.PermStore.Delete(cleaned); err != nil {
		apierror.InternalError(w, "permstore delete")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

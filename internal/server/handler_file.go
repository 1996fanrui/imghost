package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/1996fanrui/imghost/internal/apierror"
	"github.com/1996fanrui/imghost/internal/permission"
	"github.com/1996fanrui/imghost/internal/storage"
)

// FileHandler serves GET/PUT/DELETE for user file paths.
type FileHandler struct {
	DataDir   string
	FS        storage.FS
	PermStore *storage.PermStore
	Resolver  *permission.Resolver
	APIKey    string
}

func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.Get(w, r)
	case http.MethodPut:
		h.Put(w, r)
	case http.MethodDelete:
		h.Delete(w, r)
	default:
		apierror.MethodNotAllowed(w, "method not allowed")
	}
}

func (h *FileHandler) resolve(w http.ResponseWriter, r *http.Request) (cleaned, physical string, ok bool) {
	cleaned, physical, err := ResolvePath(h.DataDir, r.URL.EscapedPath())
	if err != nil {
		if errors.Is(err, ErrSymlinkEscape) {
			apierror.Forbidden(w, "symlink escape")
			return "", "", false
		}
		apierror.BadRequest(w, "invalid path")
		return "", "", false
	}
	return cleaned, physical, true
}

// Get serves a stored file.
//
// @Summary  Download file
// @Tags     file
// @Produce  application/octet-stream
// @Param    path  path   string  true  "object path (wildcard)"
// @Success  200   {file}  file   "file content"
// @Failure  401   {object}  apierror.Response  "private file without valid token"
// @Failure  403   {object}  apierror.Response  "path is a directory or symlink escape"
// @Failure  404   {object}  apierror.Response  "not found"
// @Router   /{path} [get]
func (h *FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	cleaned, physical, ok := h.resolve(w, r)
	if !ok {
		return
	}
	info, err := h.FS.Stat(physical)
	if err != nil {
		apierror.NotFound(w, "not found")
		return
	}
	if info.IsDir() {
		apierror.Forbidden(w, "directory listing not allowed")
		return
	}
	access, err := h.Resolver.Resolve(cleaned)
	if err != nil {
		apierror.InternalError(w, "resolve access")
		return
	}
	if access == permission.Private && !CheckAuth(r, h.APIKey) {
		WriteUnauthorized(w)
		return
	}
	f, err := h.FS.Open(physical)
	if err != nil {
		apierror.NotFound(w, "not found")
		return
	}
	defer func() { _ = f.Close() }()
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// Put uploads or overwrites a file.
//
// @Summary  Upload file
// @Tags     file
// @Accept   application/octet-stream
// @Produce  application/json
// @Param    path               path    string  true   "object path (wildcard)"
// @Param    Authorization      header  string  true   "Bearer <API_KEY>"
// @Param    X-Access           header  string  false  "public or private"
// @Param    body               body    string  true   "raw file bytes"
// @Success  201  {object}  map[string]string  "{\"path\":\"/<path>\"}"
// @Failure  400  {object}  apierror.Response
// @Failure  401  {object}  apierror.Response
// @Router   /{path} [put]
func (h *FileHandler) Put(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(r, h.APIKey) {
		WriteUnauthorized(w)
		return
	}
	cleaned, physical, ok := h.resolve(w, r)
	if !ok {
		return
	}
	if IsReserved(cleaned) {
		apierror.BadRequest(w, "reserved path")
		return
	}

	var newAccess permission.Access
	hasAccess := false
	if v := r.Header.Get("X-Access"); v != "" {
		a, err := permission.Parse(v)
		if err != nil {
			apierror.BadRequest(w, "invalid X-Access")
			return
		}
		newAccess = a
		hasAccess = true
	}

	oldAccess, oldExists, err := h.PermStore.Get(cleaned)
	if err != nil {
		apierror.InternalError(w, "permstore read")
		return
	}

	if hasAccess {
		if err := h.PermStore.Put(cleaned, newAccess); err != nil {
			apierror.InternalError(w, "permstore write")
			return
		}
	}

	if err := h.FS.AtomicWrite(physical, r.Body); err != nil {
		// Rollback permstore.
		if hasAccess {
			if oldExists {
				_ = h.PermStore.Put(cleaned, oldAccess)
			} else {
				_ = h.PermStore.Delete(cleaned)
			}
		}
		apierror.InternalError(w, "write file")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"path": cleaned})
}

// Delete removes a stored file.
//
// @Summary  Delete file
// @Tags     file
// @Param    path           path    string  true  "object path (wildcard)"
// @Param    Authorization  header  string  true  "Bearer <API_KEY>"
// @Success  204
// @Failure  401  {object}  apierror.Response
// @Failure  404  {object}  apierror.Response
// @Router   /{path} [delete]
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !CheckAuth(r, h.APIKey) {
		WriteUnauthorized(w)
		return
	}
	cleaned, physical, ok := h.resolve(w, r)
	if !ok {
		return
	}
	if IsReserved(cleaned) {
		apierror.BadRequest(w, "reserved path")
		return
	}
	info, err := h.FS.Stat(physical)
	if err != nil {
		apierror.NotFound(w, "not found")
		return
	}
	if info.IsDir() {
		apierror.Forbidden(w, "is a directory")
		return
	}
	if err := h.FS.Remove(physical); err != nil {
		apierror.InternalError(w, "remove failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

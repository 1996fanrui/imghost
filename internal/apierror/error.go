// Package apierror writes unified JSON error responses.
package apierror

import (
	"encoding/json"
	"net/http"
)

// Response is the unified JSON error body.
type Response struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Write emits a JSON error body with the given status, code and message.
func Write(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Error: code, Message: message})
}

func BadRequest(w http.ResponseWriter, message string) {
	Write(w, http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(w http.ResponseWriter, message string) {
	Write(w, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(w http.ResponseWriter, message string) {
	Write(w, http.StatusForbidden, "forbidden", message)
}

func NotFound(w http.ResponseWriter, message string) {
	Write(w, http.StatusNotFound, "not_found", message)
}

func MethodNotAllowed(w http.ResponseWriter, message string) {
	Write(w, http.StatusMethodNotAllowed, "method_not_allowed", message)
}

func InternalError(w http.ResponseWriter, message string) {
	Write(w, http.StatusInternalServerError, "internal_error", message)
}

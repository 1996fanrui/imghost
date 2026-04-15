// Package server contains HTTP routing, middleware and handlers.
package server

import "strings"

// ReservedPrefixes are path prefixes owned by the system; file writes targeting
// any of these are rejected. Each entry must end with "/".
var ReservedPrefixes = []string{"/swagger/"}

// ReservedExact are exact paths owned by the system; needed alongside the
// prefix form so the bare variant (e.g. "/swagger") cannot sneak through.
var ReservedExact = []string{"/swagger"}

// IsReserved reports whether the given cleaned URL path belongs to the system.
func IsReserved(path string) bool {
	for _, p := range ReservedExact {
		if path == p {
			return true
		}
	}
	for _, p := range ReservedPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

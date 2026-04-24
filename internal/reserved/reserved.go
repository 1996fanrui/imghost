// Package reserved is the single source of truth for first-segment names
// that the filehub router owns and that must not be used as a [[root]] name.
//
// Keeping this list in its own leaf package avoids the import cycle that
// would occur if both internal/config and internal/server tried to own it.
package reserved

// names lists every reserved first-segment name. Any URL whose first path
// segment equals one of these entries is routed to a dedicated handler in
// internal/server, never to the root-dispatch catch-all. Unexported because
// only IsName has real callers today.
var names = []string{"swagger"}

// IsName reports whether name is a reserved first-segment name.
func IsName(name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

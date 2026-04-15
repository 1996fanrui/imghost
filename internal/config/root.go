package config

import "github.com/1996fanrui/imghost/internal/permission"

// Root declares a URL namespace and its physical directory.
// Access is zero-value when not set in the TOML; callers use
// EffectiveAccess(def) to resolve against the config default.
type Root struct {
	Name   string            `toml:"name"`
	Path   string            `toml:"path"`
	Access permission.Access `toml:"access,omitempty"`
}

// EffectiveAccess returns the root's own access when set, otherwise def.
func (r *Root) EffectiveAccess(def permission.Access) permission.Access {
	if r.Access == "" {
		return def
	}
	return r.Access
}

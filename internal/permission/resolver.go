package permission

import (
	"fmt"
	"strings"
)

// Store retrieves an explicit access rule for a normalized path.
// Abstracted so Resolver does not depend on the bbolt implementation.
type Store interface {
	Get(path string) (Access, bool, error)
}

type Resolver struct {
	Store Store
}

// Resolve walks from path up to "/" returning the first explicit rule,
// falling back to def when nothing matches. def is supplied per call so
// the caller can pass a root-scoped effective default (per-root access
// override) instead of a single global fallback.
func (r *Resolver) Resolve(path string, def Access) (Access, error) {
	norm, err := normalize(path)
	if err != nil {
		return "", err
	}
	cur := norm
	for {
		a, ok, err := r.Store.Get(cur)
		if err != nil {
			return "", err
		}
		if ok {
			return a, nil
		}
		if cur == "/" {
			return def, nil
		}
		idx := strings.LastIndex(cur, "/")
		if idx <= 0 {
			cur = "/"
		} else {
			cur = cur[:idx]
		}
	}
}

func normalize(p string) (string, error) {
	if !strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("path %q must start with /", p)
	}
	if p == "/" {
		return p, nil
	}
	return strings.TrimRight(p, "/"), nil
}

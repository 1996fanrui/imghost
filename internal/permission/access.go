// Package permission models the access-control enum and inheritance resolver.
package permission

import "fmt"

type Access string

const (
	Public  Access = "public"
	Private Access = "private"
)

func Parse(s string) (Access, error) {
	switch Access(s) {
	case Public:
		return Public, nil
	case Private:
		return Private, nil
	default:
		return "", fmt.Errorf("invalid access %q: must be %q or %q", s, Public, Private)
	}
}

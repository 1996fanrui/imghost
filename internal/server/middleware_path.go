package server

import (
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrTraversal signals a URL-side traversal attempt (a literal ".." segment
// after decode, or a decoded path that does not remain anchored at "/").
// Callers map this to HTTP 403.
var ErrTraversal = errors.New("path traversal")

// ErrJoinEscape signals that filepath.Join of a root and the cleaned suffix
// resolves outside the root subtree. Callers map this to HTTP 403.
var ErrJoinEscape = errors.New("join escape")

// ErrSymlinkEscape signals a symlink resolves outside the root subtree.
// Callers map this to HTTP 403.
var ErrSymlinkEscape = errors.New("symlink escape")

// ResolvePath validates and canonicalizes a URL-escaped path suffix against a
// root's physical directory. It returns the cleaned URL suffix (starting with
// "/") and the corresponding physical filesystem path within the root.
//
// Validation order:
//  1. PathUnescape the raw escaped suffix.
//  2. Reject any segment equal to ".." before path.Clean.
//  3. path.Clean for canonical URL form.
//  4. filepath.Join + filepath.Rel against the root to guard join escape.
//  5. EvalSymlinks on the existing target (or nearest existing ancestor) to
//     block symlink-based escape.
func ResolvePath(rootPath, suffixEscaped string) (string, string, error) {
	decoded, err := url.PathUnescape(suffixEscaped)
	if err != nil {
		return "", "", ErrTraversal
	}

	for _, seg := range strings.Split(decoded, "/") {
		if isTraversalSegment(seg) {
			return "", "", ErrTraversal
		}
	}

	cleaned := path.Clean(decoded)
	if !strings.HasPrefix(cleaned, "/") {
		return "", "", ErrTraversal
	}

	physical := filepath.Join(rootPath, cleaned)
	if !isWithinRoot(rootPath, physical) {
		return "", "", ErrJoinEscape
	}

	if err := checkSymlinkEscape(rootPath, physical); err != nil {
		return "", "", err
	}

	return cleaned, physical, nil
}

func isTraversalSegment(seg string) bool {
	return seg == ".."
}

// checkSymlinkEscape resolves symlinks on the physical target (or its nearest
// existing ancestor when the target itself does not yet exist, e.g. PUT of a
// new file) and verifies the result is still inside rootPath.
func checkSymlinkEscape(rootPath, physical string) error {
	probe := physical
	for {
		if _, err := os.Lstat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return nil
		}
		probe = parent
	}

	evaluated, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return ErrSymlinkEscape
	}
	evaluatedRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return ErrSymlinkEscape
	}
	if !isWithinRoot(evaluatedRoot, evaluated) {
		return ErrSymlinkEscape
	}
	return nil
}

// isWithinRoot reports whether target resolves inside root using filepath.Rel.
func isWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if filepath.IsAbs(rel) {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

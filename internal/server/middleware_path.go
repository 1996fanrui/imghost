package server

import (
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrTraversal signals the request path attempted directory traversal or
// resolved outside the data root. Callers map this to HTTP 400.
var ErrTraversal = errors.New("path traversal")

// ErrSymlinkEscape signals a symlink resolves to a location outside the data
// root. Callers map this to HTTP 403.
var ErrSymlinkEscape = errors.New("symlink escape")

// ResolvePath validates and canonicalizes a URL-escaped request path against
// dataDir. It returns the cleaned URL path (starting with "/") and the
// corresponding physical filesystem path.
//
// The validation follows the exact order mandated by design.md:
//  1. PathUnescape the raw escaped path.
//  2. Reject any segment equal to ".." before path.Clean (cleaning would hide it).
//  3. path.Clean for canonical URL form.
//  4. filepath.Join + filepath.Rel to guard against escape via the join step.
//  5. EvalSymlinks on the existing target (or nearest existing ancestor) to
//     block symlink-based escape.
func ResolvePath(dataDir, urlEscapedPath string) (string, string, error) {
	decoded, err := url.PathUnescape(urlEscapedPath)
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

	physical := filepath.Join(dataDir, cleaned)
	if !isWithinRoot(dataDir, physical) {
		return "", "", ErrTraversal
	}

	if err := checkSymlinkEscape(dataDir, physical); err != nil {
		return "", "", err
	}

	return cleaned, physical, nil
}

func isTraversalSegment(seg string) bool {
	return seg == ".."
}

// checkSymlinkEscape resolves symlinks on the physical target (or its nearest
// existing ancestor when the target itself does not yet exist, e.g. PUT of a
// new file) and verifies the result is still inside dataDir.
func checkSymlinkEscape(dataDir, physical string) error {
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
	evaluatedRoot, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		return ErrSymlinkEscape
	}
	if !isWithinRoot(evaluatedRoot, evaluated) {
		return ErrSymlinkEscape
	}
	return nil
}

// isWithinRoot reports whether target resolves inside root using filepath.Rel.
// Exported via package-level tests to cover the escape branch.
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

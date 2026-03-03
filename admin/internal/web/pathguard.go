// Package web provides the HTTP server for the admin web dashboard.
package web

import (
	"path/filepath"
	"strings"
)

// ResolveSafePath resolves a relative path within a base directory.
// Returns the absolute resolved path and true when the path is safe.
// Returns empty string and false when the path escapes the base directory,
// contains "..", or is absolute.
//
// Protection strategy:
//  1. Reject absolute paths immediately.
//  2. Reject paths containing ".." before joining to prevent traversal tricks.
//  3. Join with base, resolve to absolute, verify the result starts with the
//     resolved base prefix followed by a separator.
func ResolveSafePath(baseDir, relPath string) (string, bool) {
	if filepath.IsAbs(relPath) {
		return "", false
	}
	if strings.Contains(relPath, "..") {
		return "", false
	}

	abs := filepath.Join(baseDir, relPath)
	absResolved, err := filepath.Abs(abs)
	if err != nil {
		return "", false
	}
	baseResolved, err := filepath.Abs(baseDir)
	if err != nil {
		return "", false
	}

	// Require the resolved path to be strictly inside the base directory.
	// The separator suffix prevents a base of "/foo" from matching "/foobar/baz".
	if !strings.HasPrefix(absResolved, baseResolved+string(filepath.Separator)) {
		return "", false
	}
	return absResolved, true
}

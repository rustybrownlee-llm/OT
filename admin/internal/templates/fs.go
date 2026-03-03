// Package templates embeds all HTML template files into the admin binary.
// Templates are embedded at compile time via go:embed, enabling single-binary
// deployment with no external file dependencies at runtime.
package templates

import "embed"

// FS is the embedded filesystem containing all HTML templates.
// Admin web handlers access templates via this filesystem.
//
//go:embed *.html partials/*.html
var FS embed.FS

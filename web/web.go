// Package web holds the panel's embedded dashboard. The static assets are
// compiled into the binary with go:embed, so the panel stays a single file
// with no separate front-end build step or runtime dependency.
package web

import "embed"

//go:embed static
var files embed.FS

// FS returns the embedded static asset filesystem rooted at the asset dir.
func FS() embed.FS { return files }

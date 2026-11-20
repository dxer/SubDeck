//go:build !noembed

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var embeddedFS embed.FS

func getFrontendDistFS() (fs.FS, error) {
	return fs.Sub(embeddedFS, "frontend/dist")
}

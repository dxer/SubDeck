//go:build noembed

package main

import (
	"errors"
	"io/fs"
)

func getFrontendDistFS() (fs.FS, error) {
	return nil, errors.New("frontend assets not embedded (built with noembed tag)")
}

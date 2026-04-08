package kronos

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var webDistFS embed.FS

// WebDistFS returns the embedded web/dist filesystem as an fs.FS,
// with the "web/dist" prefix stripped. Returns nil if the embedded
// directory is empty (dev mode, no frontend build).
func WebDistFS() fs.FS {
	sub, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		return nil
	}
	// Check if there's actual content (not just .gitkeep).
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}

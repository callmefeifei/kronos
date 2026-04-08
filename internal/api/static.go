package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RegisterSPA serves the embedded Vue SPA for non-API routes.
// If distFS is nil or has no index.html (dev mode), this is a no-op
// except for the API 404 handler.
func RegisterSPA(r *gin.Engine, distFS fs.FS) {
	if distFS == nil {
		// Even without SPA, set NoRoute to return JSON for API paths.
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
		})
		return
	}

	// Check if there's an index.html — if not, we're in dev mode with empty dist.
	_, err := fs.Stat(distFS, "index.html")
	if err != nil {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
		})
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	// Serve static assets directly.
	r.GET("/assets/*filepath", gin.WrapH(fileServer))
	r.GET("/vite.svg", gin.WrapH(fileServer))
	r.GET("/favicon.ico", gin.WrapH(fileServer))

	// SPA fallback: any non-API route serves index.html.
	r.NoRoute(func(c *gin.Context) {
		// Don't intercept API routes — return 404 JSON for those.
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "not found"})
			return
		}

		indexHTML, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "index.html not found"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
}

package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func SpaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the executable directory
		execPath, err := os.Executable()
		if err != nil {
			execPath, _ = os.Getwd()
		}
		execDir := filepath.Dir(execPath)

		// Try multiple possible locations for the web/dist directory
		possibleDirs := []string{
			filepath.Join(execDir, "web", "dist"),
			filepath.Join(execDir, "..", "web", "dist"),
			"./web/dist",
		}

		var distDir string
		for _, dir := range possibleDirs {
			if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
				distDir = dir
				break
			}
		}

		if distDir == "" {
			// Fallback - serve a simple message
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>AwareNow</title></head>
<body>
<h1>AwareNow</h1>
<p>Frontend not built. Run <code>npm run build</code> in the web/ directory.</p>
</body>
</html>`))
			return
		}

		// Clean the path
		path := filepath.Clean(r.URL.Path)

		// Prevent path traversal using filepath.Rel for robust validation
		relPath, err := filepath.Rel(distDir, filepath.Join(distDir, path))
		if err != nil || strings.HasPrefix(relPath, "..") {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		// Check if the requested file exists
		filePath := filepath.Join(distDir, relPath)
		info, err := os.Stat(filePath)
		if err == nil && !info.IsDir() {
			// File exists, serve it
			http.ServeFile(w, r, filePath)
			return
		}

		// File doesn't exist or is a directory, serve index.html for client-side routing
		http.ServeFile(w, r, filepath.Join(distDir, "index.html"))
	})
}

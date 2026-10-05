package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Camouflage serves a real public page without exposing secret panel paths.
// A configured directory may replace it; directories are never listed.
func Camouflage(directory string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			NotFound(w)
			return
		}
		if directory != "" {
			name := strings.TrimPrefix(r.URL.Path, "/")
			if name == "" {
				name = "index.html"
			}
			root, err := os.OpenRoot(directory)
			if err == nil {
				defer root.Close()
				file, err := root.Open(name)
				if err == nil {
					defer file.Close()
					if info, err := file.Stat(); err == nil && info.Mode().IsRegular() {
						http.ServeContent(w, r, filepath.Base(name), info.ModTime(), file)
						return
					}
				}
			}
		}
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
		case "/favicon.ico":
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="10" fill="#ede9f8"/><circle cx="16" cy="16" r="7" fill="#a7bcb4"/></svg>`))
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if r.URL.Path != "/" {
				w.WriteHeader(http.StatusNotFound)
			}
			_, _ = w.Write([]byte(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Service portal</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:linear-gradient(130deg,#f8e9dd,#ece9f6,#e0eeeb);color:#222936;font:16px system-ui}main{max-width:500px;padding:48px;border:1px solid white;border-radius:32px;background:#ffffffb0;box-shadow:0 18px 70px #26344212}small{letter-spacing:.16em;color:#62736d}h1{font-size:42px;letter-spacing:-.04em}p{line-height:1.7;color:#65707f}footer{margin-top:40px;font-size:13px;color:#65707f}</style><main><small>CONNECTED SERVICES</small><h1>A quieter digital space.</h1><p>Our services are running. This portal provides a secure entry point for registered customers.</p><footer>Service portal · Secure connections</footer></main></html>`))
		}
	})
}

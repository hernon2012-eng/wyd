package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var staticFiles embed.FS

func Handler() http.Handler {
	files, err := fs.Sub(staticFiles, "assets")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		fileServer.ServeHTTP(w, r)
	})
}

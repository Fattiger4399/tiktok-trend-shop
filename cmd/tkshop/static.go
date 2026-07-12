package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// newStaticServer returns a handler that serves files from dir while guarding
// against path traversal and never exposing files outside the configured
// directory.
func newStaticServer(dir string) (http.Handler, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrInvalid
	}
	fs := http.FileServer(http.Dir(abs))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		if strings.Contains(clean, "..") {
			http.NotFound(w, r)
			return
		}
		full := filepath.Join(abs, clean)
		if !strings.HasPrefix(full, abs) {
			http.NotFound(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	}), nil
}
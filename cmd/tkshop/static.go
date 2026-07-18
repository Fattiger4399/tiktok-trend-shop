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
	return newGuardedFileServer(abs), nil
}

// newAssetsServer serves the mediagen storage root under /assets/ with the
// same traversal guard as newStaticServer. The root is created when missing
// so the API can boot before the first asset is generated.
func newAssetsServer(dir string) (http.Handler, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return http.StripPrefix("/assets/", newGuardedFileServer(abs)), nil
}

// newGuardedFileServer serves files from abs (an existing directory) while
// guarding against path traversal and never exposing files outside of it.
func newGuardedFileServer(abs string) http.Handler {
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
	})
}
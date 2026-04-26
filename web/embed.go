// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

func Handler() http.Handler {
	if os.Getenv("OPENARTIFACT_DEV") != "" {
		target, _ := url.Parse("http://localhost:5173")
		return httputil.NewSingleHostReverseProxy(target)
	}

	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: failed to create sub FS: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}

		_, statErr := fs.Stat(sub, clean)
		if statErr == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		index, readErr := fs.ReadFile(sub, "index.html")
		if readErr != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
}

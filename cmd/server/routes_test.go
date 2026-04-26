// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestRouteRegistrationOrderNoPanic mirrors the route-registration order used
// in main() to guard against chi Mount() panicking when a catch-all is registered on the same prefix a
// subrouter is later mounted on.
func TestRouteRegistrationOrderNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("route registration panicked: %v", r)
		}
	}()

	h := func(w http.ResponseWriter, r *http.Request) {}
	r := chi.NewRouter()

	// Order mirrors cmd/server/main.go: subrouter mount first, then the
	// npm publish/unpublish catch-all on the same /repos prefix.
	r.Route("/repos/{name}", func(sub chi.Router) {
		sub.Get("/*", h)
	})
	r.Put("/repos/{repo}/*", h)
	r.Delete("/repos/{repo}/*", h)
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package urlparams

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Vars returns all URL path parameters from a chi request context as a map.
// This is a drop-in replacement for mux.Vars(r).
func Vars(r *http.Request) map[string]string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return map[string]string{}
	}

	m := make(map[string]string, len(rctx.URLParams.Keys))

	for i, k := range rctx.URLParams.Keys {
		m[k] = rctx.URLParams.Values[i]
	}

	return m
}

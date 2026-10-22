// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The android management API is authorized with the repository's own
// read/write/delete permissions (so tags and notes need only write), never
// the global manage permission.
func TestClassifyRouteAndroid(t *testing.T) {
	const base = "/api/v1/repositories/phone/apks"

	app := func(action string) routeScope { return routeScope{action: action, repoName: "phone"} }

	for _, tc := range []struct {
		method, path string
		want         routeScope
	}{
		{http.MethodGet, base + "/apps", app("read")},
		{http.MethodGet, base + "/versions/9/icon", app("read")},
		{http.MethodPost, base, app("write")},
		{http.MethodPost, base + "/inspect", app("write")},
		{http.MethodPut, base + "/apps/org.example/meta", app("write")},
		{http.MethodPut, base + "/versions/9/meta", app("write")},
		{http.MethodPost, base + "/signers", app("write")},
		{http.MethodDelete, base + "/versions/9", app("delete")},
		{http.MethodDelete, base + "/signers/9", app("delete")},
		{http.MethodPatch, base + "/versions/9/meta", routeScope{action: "manage"}},
		{http.MethodGet, "/api/v1/repositories/a%2Fb/apks/apps", routeScope{action: "manage"}},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)

			if got := classifyRoute(req); got != tc.want {
				t.Errorf("classifyRoute = %+v, want %+v", got, tc.want)
			}
		})
	}
}

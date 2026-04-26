// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import "net/http"

// isSecureRequest returns true when the request appears to be served over
// TLS — directly or through a reverse proxy that sets X-Forwarded-Proto.
//
// The Set-Cookie Secure attribute can only be honoured by the browser when
// the cookie is delivered over a secure context. Hard-coding Secure: true
// makes plain-HTTP development (e.g. http://localhost) silently drop the
// cookie, breaking session login.
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
		return true
	}

	return false
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/audit"
)

func AuditMiddleware(auditLogger *audit.Logger, trusted ...*TrustedProxies) func(http.Handler) http.Handler {
	var proxies *TrustedProxies
	if len(trusted) > 0 {
		proxies = trusted[0]
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
				return
			}

			routeTemplate := "unknown"

			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if p := rctx.RoutePattern(); p != "" {
					routeTemplate = strings.TrimSuffix(p, "/*")
				}
			}

			actor := "anonymous"
			if p := PrincipalFromContext(r.Context()); p != nil && !p.IsAnonymous {
				actor = p.Username
			}

			action := routeToAction(r.Method, routeTemplate)
			if action == "" {
				action = "http.request"
			}
			clientIP := ClientIP(r, proxies)

			details := fmt.Sprintf(`{"method":"%s","path":"%s","status":%d}`, r.Method, routeTemplate, wrapped.statusCode)
			if wrapped.statusCode >= 400 {
				details = fmt.Sprintf(`{"method":"%s","path":"%s","status":%d,"error":true}`, r.Method, routeTemplate, wrapped.statusCode)
			}

			auditLogger.Log(r.Context(), audit.AuditEvent{
				Action:    action,
				Actor:     actor,
				Details:   details,
				IPAddress: &clientIP,
			})
		})
	}
}

func routeToAction(method, routeTemplate string) string {
	switch {
	case method == "POST" && routeTemplate == "/api/v1/repositories":
		return "repository.created"
	case method == "DELETE" && routeTemplate == "/api/v1/repositories/{name}":
		return "repository.deleted"
	case method == "POST" && routeTemplate == "/api/v1/repositories/{name}/debs":
		return "artifact.uploaded"
	case method == "PUT" && routeTemplate == "/repos/{repo}/{pkg:.*}":
		return "artifact.uploaded"
	case method == "POST" && routeTemplate == "/api/v1/auth/login":
		return "auth.login"
	}
	return ""
}

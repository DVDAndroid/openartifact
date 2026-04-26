// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package middleware

import (
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/openartifact/openartifact/pkg/logger"
)

// RequestIDMiddleware adds a unique request ID to each request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		r.Header.Set("X-Request-ID", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests.
func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := r.Header.Get("X-Request-ID")

			// Create a response writer wrapper to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			log.Infow("HTTP request",
				"request_id", requestID,
				"method", r.Method,
				"path", r.RequestURI,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// RecoveryMiddleware recovers from panics.
func RecoveryMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := r.Header.Get("X-Request-ID")
					log.Errorw("panic recovered",
						"request_id", requestID,
						"error", err,
						"path", r.RequestURI,
					)
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal Server Error"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware handles CORS headers.
//
// The browser refuses to deliver a credentialed response when the server
// returns Access-Control-Allow-Origin: * — credentials require an exact
// origin echo plus Access-Control-Allow-Credentials: true. The SPA always
// uses fetch with credentials: 'include' (for the session cookie), so we
// echo the request Origin when it is allowed and advertise credentials.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	const (
		allowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
		// X-Requested-With is sent by the SPA's api client. Without it the
		// preflight rejects every real request.
		allowedHeaders = "Content-Type, Authorization, X-Requested-With, X-API-Key"
	)

	wildcard := slices.Contains(allowedOrigins, "*")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Vary tells caches the response depends on the Origin header.
			w.Header().Add("Vary", "Origin")

			allowed := false

			if origin != "" {
				if wildcard {
					allowed = true
				} else {
					allowed = slices.Contains(allowedOrigins, origin)
				}
			}

			if allowed {
				// Echo the origin so credentialed responses are accepted.
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter

	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

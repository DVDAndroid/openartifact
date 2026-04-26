// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newMetricsRouter(token string) chi.Router {
	router := chi.NewRouter()
	authMW := metricsAuthMW(token)
	pr := prometheus.NewRegistry()
	router.Handle("/metrics", authMW(promhttp.HandlerFor(pr, promhttp.HandlerOpts{})))

	return router
}

func metricsAuthMW(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if len(auth) < 7 || auth[:7] != "Bearer " || auth[7:] != token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func TestMetricsEndpoint_NoToken_Open(t *testing.T) {
	router := newMetricsRouter("")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMetricsEndpoint_WrongToken_401(t *testing.T) {
	router := newMetricsRouter("secret")
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/metrics", nil)
	req.Header.Set("Authorization", "Bearer wrong")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMetricsEndpoint_CorrectToken_200(t *testing.T) {
	router := newMetricsRouter("secret")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

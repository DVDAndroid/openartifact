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
	"github.com/openartifact/openartifact/internal/metrics"
	"github.com/openartifact/openartifact/internal/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsMiddleware_CounterIncrementsWithRouteTemplate(t *testing.T) {
	pr := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "openartifact_http_requests_total",
		Help: "Total HTTP requests.",
	}, []string{"method", "path", "status"})
	histogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "openartifact_http_request_duration_seconds",
		Help:    "HTTP request duration.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})
	pr.MustRegister(counter, histogram)

	reg := &metrics.Registry{
		HTTPRequestsTotal:          counter,
		HTTPRequestDurationSeconds: histogram,
	}

	router := chi.NewRouter()
	router.Use(middleware.MetricsMiddleware(reg))
	router.HandleFunc("/repos/{name}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/repos/foo", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	val := testutil.ToFloat64(counter.WithLabelValues("GET", "/repos/{name}", "200"))
	if val != 1 {
		t.Errorf("expected counter=1 for GET /repos/{name} 200, got %v", val)
	}
}

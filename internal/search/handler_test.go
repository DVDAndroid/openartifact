// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/middleware"
	"github.com/openartifact/openartifact/pkg/logger"
)

type mockSearcher struct {
	count   int64
	results []SearchResult
	err     error
}

func (m *mockSearcher) CountResults(_ context.Context, _ SearchFilter, _ map[uuid.UUID]struct{}) (int64, error) {
	return m.count, m.err
}

func (m *mockSearcher) Search(_ context.Context, _ SearchFilter, _, _ int, _ map[uuid.UUID]struct{}) ([]SearchResult, error) {
	return m.results, m.err
}

func newTestHandler(t *testing.T, ms *mockSearcher) (*Handler, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}

	mock.ExpectQuery(`SELECT id, name FROM repositories`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	log, _ := logger.New("error", "json", "stderr")
	ev := auth.NewEvaluator(nil, log, true)
	h := &Handler{
		searcher:  ms,
		db:        db,
		evaluator: ev,
		log:       log,
	}

	return h, func() { _ = db.Close() }
}

func adminPrincipal() *auth.Principal {
	id := "test-user-id"
	return &auth.Principal{UserID: &id, Username: "admin", IsLegacy: true}
}

func requestWithPrincipal(method, target string, p *auth.Principal) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), method, target, nil)
	if p != nil {
		r = r.WithContext(middleware.WithPrincipal(r.Context(), p))
	}

	return r
}

func TestHandlerMissingQ(t *testing.T) {
	h, cleanup := newTestHandler(t, &mockSearcher{})
	defer cleanup()

	w := httptest.NewRecorder()
	r := requestWithPrincipal("GET", "/api/v1/search", adminPrincipal())
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandlerLimitTooHigh(t *testing.T) {
	h, cleanup := newTestHandler(t, &mockSearcher{count: 1})
	defer cleanup()

	w := httptest.NewRecorder()
	r := requestWithPrincipal("GET", "/api/v1/search?q=test&limit=200", adminPrincipal())
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandlerCountTooHigh(t *testing.T) {
	h, cleanup := newTestHandler(t, &mockSearcher{count: 10001})
	defer cleanup()

	w := httptest.NewRecorder()
	r := requestWithPrincipal("GET", "/api/v1/search?q=test", adminPrincipal())
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandlerSuccessfulSearch(t *testing.T) {
	ms := &mockSearcher{
		count: 1,
		results: []SearchResult{
			{Repository: "my-repo", Format: "npm", Name: "lodash", Version: "4.17.21", CreatedAt: time.Now()},
		},
	}

	h, cleanup := newTestHandler(t, ms)
	defer cleanup()

	db, mock2, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	h.db = db

	repoID := uuid.New().String()
	mock2.ExpectQuery(`SELECT id, name FROM repositories`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(repoID, "my-repo"))

	w := httptest.NewRecorder()
	r := requestWithPrincipal("GET", "/api/v1/search?q=lodash", adminPrincipal())
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["total"] == nil {
		t.Error("missing 'total' in response")
	}

	if resp["results"] == nil {
		t.Error("missing 'results' in response")
	}
}

func TestHandlerEmptyResult(t *testing.T) {
	ms := &mockSearcher{count: 0, results: nil}

	h, cleanup := newTestHandler(t, ms)
	defer cleanup()

	db, mock2, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	h.db = db

	mock2.ExpectQuery(`SELECT id, name FROM repositories`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

	w := httptest.NewRecorder()
	r := requestWithPrincipal("GET", "/api/v1/search?q=nothing", adminPrincipal())
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]any

	_ = json.NewDecoder(w.Body).Decode(&resp)

	results, ok := resp["results"].([]any)
	if !ok {
		t.Fatalf("results not found or not an array")
	}

	if len(results) != 0 {
		t.Errorf("want empty results, got %v", results)
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package group

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()

	log, err := logger.New("error", "json", "stderr")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	return NewHandler(log, nil, nil)
}

// TestServeGroup_LaterMemberSucceedsAfterEarlier404 verifies that a
// first member's 404 is not written to the real ResponseWriter, so a later
// member's 200 response is not preceded by it or concatenated with its body.
// Attempts must be buffered so only the winning one is flushed.
func TestServeGroup_LaterMemberSucceedsAfterEarlier404(t *testing.T) {
	h := newTestHandler(t)

	member1 := &models.Repository{ID: "m1", RepoType: "hosted"}
	member2 := &models.Repository{ID: "m2", RepoType: "hosted"}

	serveRepo := func(w http.ResponseWriter, r *http.Request, repo *models.Repository) {
		if repo.ID == "m1" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("X-Served-By", repo.ID)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok from " + repo.ID))
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeGroup(w, req, []*models.Repository{member1, member2}, serveRepo)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (before the fix this leaked as 404)", w.Code)
	}

	if got := w.Body.String(); got != "ok from m2" {
		t.Fatalf("body = %q, want %q (must not be concatenated with the 404 body)", got, "ok from m2")
	}

	if got := w.Header().Get("X-Served-By"); got != "m2" {
		t.Fatalf("X-Served-By = %q, want m2", got)
	}
}

func TestServeGroup_AllMembers404(t *testing.T) {
	h := newTestHandler(t)
	member := &models.Repository{ID: "m1", RepoType: "hosted"}

	serveRepo := func(w http.ResponseWriter, r *http.Request, _ *models.Repository) {
		http.NotFound(w, r)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeGroup(w, req, []*models.Repository{member}, serveRepo)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestServeGroup_NonNotFoundErrorStopsFanout(t *testing.T) {
	h := newTestHandler(t)
	member1 := &models.Repository{ID: "m1", RepoType: "hosted"}
	member2 := &models.Repository{ID: "m2", RepoType: "hosted"}

	calledSecond := false
	serveRepo := func(w http.ResponseWriter, _ *http.Request, repo *models.Repository) {
		if repo.ID == "m1" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}

		calledSecond = true
		w.WriteHeader(http.StatusOK)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeGroup(w, req, []*models.Repository{member1, member2}, serveRepo)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}

	if calledSecond {
		t.Fatal("second member should not have been tried after a non-404 terminal error")
	}
}

// TestServeGroup_RecursionGuard covers a group repository whose member list
// (directly or transitively) includes itself, simulating what a caller's
// serveRepo does when a member turns out to be a "group" repo too.
func TestServeGroup_RecursionGuard(t *testing.T) {
	h := newTestHandler(t)
	member := &models.Repository{ID: "self", RepoType: "group"}

	var serveRepo func(w http.ResponseWriter, r *http.Request, repo *models.Repository)

	serveRepo = func(w http.ResponseWriter, r *http.Request, _ *models.Repository) {
		h.ServeGroup(w, r, []*models.Repository{member}, serveRepo)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeGroup(w, req, []*models.Repository{member}, serveRepo)

	if w.Code != http.StatusLoopDetected {
		t.Fatalf("status = %d, want %d (loop detected)", w.Code, http.StatusLoopDetected)
	}
}

// trackingWriter wraps a ResponseWriter and records each Write call's byte
// length, so tests can assert a member's body streams through in the same
// chunks the handler wrote it, rather than being fully buffered and flushed
// as one consolidated write.
type trackingWriter struct {
	http.ResponseWriter

	writes []int
}

func (t *trackingWriter) Write(b []byte) (int, error) {
	t.writes = append(t.writes, len(b))
	return t.ResponseWriter.Write(b)
}

// TestServeGroup_StreamsWithoutFullBuffering guards against
// full in-memory buffering of a member's response body (an
// httptest.ResponseRecorder per attempt would materialize the whole body
// before any of it reaches the client). Each chunk the member writes
// must arrive at the real writer as its own Write call.
func TestServeGroup_StreamsWithoutFullBuffering(t *testing.T) {
	h := newTestHandler(t)
	member := &models.Repository{ID: "m1", RepoType: "hosted"}

	chunks := [][]byte{[]byte("chunk-one-"), []byte("chunk-two-"), []byte("chunk-three")}

	serveRepo := func(w http.ResponseWriter, _ *http.Request, _ *models.Repository) {
		w.WriteHeader(http.StatusOK)

		for _, c := range chunks {
			_, _ = w.Write(c)
		}
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	tw := &trackingWriter{ResponseWriter: rec}

	h.ServeGroup(tw, req, []*models.Repository{member}, serveRepo)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if got, want := rec.Body.String(), "chunk-one-chunk-two-chunk-three"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}

	if len(tw.writes) != len(chunks) {
		t.Fatalf("writes = %d calls, want %d — each chunk must stream straight through, "+
			"never get buffered and re-flushed as one write", len(tw.writes), len(chunks))
	}
}

// TestServeGroup_FailedMemberHeadersDoNotLeak ensures a discarded (404)
// member's headers never bleed into the response a later, successful
// member commits — each attempt must get its own isolated header set.
func TestServeGroup_FailedMemberHeadersDoNotLeak(t *testing.T) {
	h := newTestHandler(t)
	member1 := &models.Repository{ID: "m1", RepoType: "hosted"}
	member2 := &models.Repository{ID: "m2", RepoType: "hosted"}

	serveRepo := func(w http.ResponseWriter, r *http.Request, repo *models.Repository) {
		if repo.ID == "m1" {
			w.Header().Set("X-From-Failed-Member", "m1")
			http.NotFound(w, r)

			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	h.ServeGroup(w, req, []*models.Repository{member1, member2}, serveRepo)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if got := w.Header().Get("X-From-Failed-Member"); got != "" {
		t.Fatalf("X-From-Failed-Member leaked onto the final response: %q", got)
	}
}

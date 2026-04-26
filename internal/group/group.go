// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package group

import (
	"context"
	"net/http"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/storage"
	"github.com/openartifact/openartifact/pkg/logger"
)

type Handler struct {
	log       *logger.Logger
	db        database.Database
	blobStore storage.BlobStore
}

func NewHandler(log *logger.Logger, db database.Database, blobStore storage.BlobStore) *Handler {
	return &Handler{
		log:       log,
		db:        db,
		blobStore: blobStore,
	}
}

// groupDepthKey is the context key used to detect group-in-group recursion:
// a "group" repo can itself list another "group" repo as a member, and
// serveRepo re-entering ServeGroup for that nested group would otherwise
// recurse without bound. The counter travels on the *http.Request context
// through however many layers of indirection the caller's serveRepo uses.
type groupDepthKey struct{}

// maxGroupDepth bounds group-of-groups nesting.
const maxGroupDepth = 8

// ServeGroup handles a request for a group repository.
// members is the ordered list of member repos (by priority asc).
// serveRepo is a callback that serves a request for a specific repo.
func (h *Handler) ServeGroup(w http.ResponseWriter, r *http.Request, members []*models.Repository, serveRepo func(w http.ResponseWriter, r *http.Request, repo *models.Repository)) {
	depth, _ := r.Context().Value(groupDepthKey{}).(int)
	if depth >= maxGroupDepth {
		h.log.Errorw("group repository nesting exceeded max depth, aborting", "depth", depth)
		http.Error(w, "group repository nesting too deep", http.StatusLoopDetected)

		return
	}

	r = r.WithContext(context.WithValue(r.Context(), groupDepthKey{}, depth+1))

	// For GET and HEAD requests: try each member in priority order, return the first
	// successful response. Each attempt runs behind a probeWriter that withholds the
	// status/headers from the real ResponseWriter until WriteHeader is observed: a 404
	// is swallowed (its body is discarded, not buffered) so the next member can still
	// win, while any other status commits immediately and streams the body straight
	// through to the real writer — nothing is ever fully materialized in memory.
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		for _, member := range members {
			pw := newProbeWriter(w)
			serveRepo(pw, r, member)

			// 404: this attempt never reached the real writer. Try the next member.
			if pw.skipped {
				continue
			}

			// 2xx, or any other terminal (non-404) status: already streamed. Done.
			return
		}

		// No member had the artifact.
		http.NotFound(w, r)

		return
	}

	// For PUT/POST/DELETE: route to the first writable (hosted) member repo
	for _, member := range members {
		if member.RepoType == "hosted" {
			serveRepo(w, r, member)
			return
		}
	}

	// No writable member found
	http.Error(w, "no writable repository member found", http.StatusBadRequest)
}

// probeWriter wraps the real http.ResponseWriter for one group-member attempt.
// It withholds status/headers until WriteHeader is called: a 404 marks the
// attempt "skipped" and every subsequent Write is discarded (never buffered,
// never reaches the real writer) so the caller can try the next member. Any
// other status commits immediately — headers and status flush to the real
// writer right away, and every subsequent Write streams straight through —
// so a multi-GB artifact body is never held in memory.
type probeWriter struct {
	real        http.ResponseWriter
	header      http.Header
	wroteHeader bool
	committed   bool
	skipped     bool
}

func newProbeWriter(w http.ResponseWriter) *probeWriter {
	return &probeWriter{real: w, header: make(http.Header)}
}

func (p *probeWriter) Header() http.Header {
	if p.committed {
		return p.real.Header()
	}

	return p.header
}

func (p *probeWriter) WriteHeader(code int) {
	if p.wroteHeader {
		return
	}

	p.wroteHeader = true

	if code == http.StatusNotFound {
		p.skipped = true
		return
	}

	dst := p.real.Header()
	for k, vals := range p.header {
		for _, v := range vals {
			dst.Add(k, v)
		}
	}

	p.real.WriteHeader(code)
	p.committed = true
}

func (p *probeWriter) Write(b []byte) (int, error) {
	if !p.wroteHeader {
		p.WriteHeader(http.StatusOK)
	}

	if p.skipped {
		return len(b), nil
	}

	return p.real.Write(b)
}

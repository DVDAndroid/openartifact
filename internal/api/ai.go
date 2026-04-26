// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/internal/worker"
	"github.com/openartifact/openartifact/pkg/logger"
)

// AIHandler handles AI analysis REST endpoints.
type AIHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
	aiWorker  *worker.AIAnalysisWorker
}

// NewAIHandler creates a new AI analysis handler.
func NewAIHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator, aiWorker *worker.AIAnalysisWorker) *AIHandler {
	return &AIHandler{db: db, log: log, evaluator: evaluator, aiWorker: aiWorker}
}

// RegisterRoutes registers AI analysis routes.
func (h *AIHandler) RegisterRoutes(router chi.Router, prefix string) {
	// GET /api/v1/repositories/{name}/ai-analysis — list analyses for a repo
	// GET /api/v1/repositories/{name}/ai-analysis/{artifactID} — get one analysis
	// POST /api/v1/repositories/{name}/ai-analysis/{artifactID}/trigger — trigger re-analysis
	router.Route(prefix, func(r chi.Router) {
		r.Get("/ai-analysis", h.handleList)
		r.Get("/ai-analysis/{artifactID}", h.handleGet)
		r.Post("/ai-analysis/{artifactID}/trigger", h.handleTrigger)
	})
}

func (h *AIHandler) handleList(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]

	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil || repo == nil {
		http.Error(w, `{"error":"repository not found"}`, http.StatusNotFound)
		return
	}

	status := r.URL.Query().Get("status")
	limit := 20
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	analyses, total, err := h.db.ListAIAnalyses(r.Context(), repo.ID, status, limit, offset)
	if err != nil {
		h.log.Errorw("ai handler: list analyses failed", "repo", repoName, "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"analyses": analyses,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *AIHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	artifactID := vars["artifactID"]

	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil || repo == nil {
		http.Error(w, `{"error":"repository not found"}`, http.StatusNotFound)
		return
	}

	analysis, err := h.db.GetAIAnalysis(r.Context(), repo.ID, artifactID)
	if err != nil {
		h.log.Errorw("ai handler: get analysis failed", "artifact_id", artifactID, "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)

		return
	}

	if analysis == nil {
		http.Error(w, `{"error":"analysis not found"}`, http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, analysis)
}

func (h *AIHandler) handleTrigger(w http.ResponseWriter, r *http.Request) {
	vars := urlparams.Vars(r)
	repoName := vars["name"]
	artifactID := vars["artifactID"]

	if h.aiWorker == nil {
		http.Error(w, `{"error":"AI analysis is not enabled (AI_ENABLED=false)"}`, http.StatusServiceUnavailable)
		return
	}

	repo, err := h.db.GetRepositoryByName(r.Context(), repoName)
	if err != nil || repo == nil {
		http.Error(w, `{"error":"repository not found"}`, http.StatusNotFound)
		return
	}

	var body struct {
		ManifestText    string `json:"manifest_text"`
		ArtifactFormat  string `json:"artifact_format"`
		ArtifactName    string `json:"artifact_name"`
		ArtifactVersion string `json:"artifact_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	h.aiWorker.Notify(worker.AIAnalysisRequest{

		RepositoryID:    repo.ID,
		ArtifactID:      artifactID,
		ArtifactFormat:  body.ArtifactFormat,
		ArtifactName:    body.ArtifactName,
		ArtifactVersion: body.ArtifactVersion,
		ManifestText:    body.ManifestText,
	})

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "queued",
		"message": "AI analysis has been queued",
	})
}

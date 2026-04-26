// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

type BundlesHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewBundlesHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *BundlesHandler {
	return &BundlesHandler{db: db, log: log, evaluator: evaluator}
}

func (h *BundlesHandler) RegisterAuthed(r chi.Router) {
	// Pipelines
	r.Get("/api/v1/pipelines", h.handleListPipelines)
	r.Post("/api/v1/pipelines", h.handleCreatePipeline)
	r.Get("/api/v1/pipelines/{name}", h.handleGetPipeline)
	r.Patch("/api/v1/pipelines/{name}", h.handleUpdatePipeline)
	r.Delete("/api/v1/pipelines/{name}", h.handleDeletePipeline)

	// Bundles
	r.Get("/api/v1/bundles", h.handleListBundles)
	r.Post("/api/v1/bundles", h.handleCreateBundle)
	r.Get("/api/v1/bundles/{name}/{version}", h.handleGetBundle)
	r.Delete("/api/v1/bundles/{name}/{version}", h.handleDeleteBundle)

	// Bundle items
	r.Get("/api/v1/bundles/{name}/{version}/items", h.handleListItems)
	r.Post("/api/v1/bundles/{name}/{version}/items", h.handleAddItems)
	r.Delete("/api/v1/bundles/{name}/{version}/items/{sha256}", h.handleRemoveItem)

	// Bundle lifecycle
	r.Post("/api/v1/bundles/{name}/{version}/seal", h.handleSeal)
	r.Post("/api/v1/bundles/{name}/{version}/promote", h.handlePromote)
	r.Post("/api/v1/bundles/{name}/{version}/archive", h.handleArchive)

	// Promotion history
	r.Get("/api/v1/bundles/{name}/{version}/promotions", h.handleListPromotions)
}

// ============================================================================
// Pipeline Handlers
// ============================================================================

func (h *BundlesHandler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	pipelines, err := h.db.ListPromotionPipelines(ctx)
	if err != nil {
		h.log.Errorw("list pipelines: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if pipelines == nil {
		pipelines = []*models.PromotionPipeline{}
	}

	var result []map[string]any
	for _, pipeline := range pipelines {
		result = append(result, pipelineToMap(pipeline))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pipelines": result,
	})
}

func (h *BundlesHandler) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	var body struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Stages      []models.PipelineStage `json:"stages"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}

	stagesJSON, err := json.Marshal(body.Stages)
	if err != nil {
		h.log.Errorw("create pipeline: marshal stages error", "error", err)
		writeError(w, http.StatusBadRequest, "bad_request", "invalid stages")

		return
	}

	pipeline := &models.PromotionPipeline{
		Name:        body.Name,
		Description: body.Description,
		Stages:      body.Stages,
		StagesRaw:   stagesJSON,
		CreatedBy:   p.Username,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.db.CreatePromotionPipeline(ctx, pipeline); err != nil {
		h.log.Errorw("create pipeline: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, pipelineToMap(pipeline))
}

func (h *BundlesHandler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	pipeline, err := h.db.GetPromotionPipelineByName(ctx, name)
	if err != nil {
		h.log.Errorw("get pipeline: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if pipeline == nil {
		writeError(w, http.StatusNotFound, "not_found", "pipeline not found")
		return
	}

	// Unmarshal stages
	if len(pipeline.StagesRaw) > 0 {
		if err := json.Unmarshal(pipeline.StagesRaw, &pipeline.Stages); err != nil {
			h.log.Errorw("get pipeline: unmarshal stages error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}
	}

	writeJSON(w, http.StatusOK, pipelineToMap(pipeline))
}

func (h *BundlesHandler) handleUpdatePipeline(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	pipeline, err := h.db.GetPromotionPipelineByName(ctx, name)
	if err != nil {
		h.log.Errorw("update pipeline: db error getting pipeline", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if pipeline == nil {
		writeError(w, http.StatusNotFound, "not_found", "pipeline not found")
		return
	}

	var body struct {
		Description *string                 `json:"description"`
		Stages      *[]models.PipelineStage `json:"stages"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Description != nil {
		pipeline.Description = *body.Description
	}

	if body.Stages != nil {
		pipeline.Stages = *body.Stages

		stagesJSON, err := json.Marshal(pipeline.Stages)
		if err != nil {
			h.log.Errorw("update pipeline: marshal stages error", "error", err)
			writeError(w, http.StatusBadRequest, "bad_request", "invalid stages")

			return
		}

		pipeline.StagesRaw = stagesJSON
	}

	pipeline.UpdatedAt = time.Now()

	if err := h.db.UpdatePromotionPipeline(ctx, pipeline); err != nil {
		h.log.Errorw("update pipeline: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, pipelineToMap(pipeline))
}

func (h *BundlesHandler) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	pipeline, err := h.db.GetPromotionPipelineByName(ctx, name)
	if err != nil {
		h.log.Errorw("delete pipeline: db error getting pipeline", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if pipeline == nil {
		writeError(w, http.StatusNotFound, "not_found", "pipeline not found")
		return
	}

	if err := h.db.DeletePromotionPipeline(ctx, pipeline.ID); err != nil {
		h.log.Errorw("delete pipeline: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

// ============================================================================
// Bundle Handlers
// ============================================================================

func (h *BundlesHandler) handleListBundles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	nameFilter := q.Get("name")
	stateFilter := q.Get("state")
	page := parseIntMin(q.Get("page"), 1, 1)
	limit := parseIntClamp(q.Get("limit"), 25, 1, 100)

	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundles, total, err := h.db.ListReleaseBundles(ctx, nameFilter, stateFilter, page, limit)
	if err != nil {
		h.log.Errorw("list bundles: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundles == nil {
		bundles = []*models.ReleaseBundle{}
	}

	var result []map[string]any
	for _, bundle := range bundles {
		result = append(result, bundleToMap(bundle))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":   total,
		"page":    page,
		"limit":   limit,
		"bundles": result,
	})
}

func (h *BundlesHandler) handleCreateBundle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	var body struct {
		Name        string  `json:"name"`
		Version     string  `json:"version"`
		Description string  `json:"description"`
		PipelineID  *string `json:"pipeline_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Name == "" || body.Version == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name and version are required")
		return
	}

	bundle := &models.ReleaseBundle{
		Name:        body.Name,
		Version:     body.Version,
		Description: body.Description,
		State:       models.BundleStateDraft,
		PipelineID:  body.PipelineID,
		CreatedBy:   p.Username,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.db.CreateReleaseBundle(ctx, bundle); err != nil {
		h.log.Errorw("create bundle: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, bundleToMap(bundle))
}

func (h *BundlesHandler) handleGetBundle(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("get bundle: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	writeJSON(w, http.StatusOK, bundleToMap(bundle))
}

func (h *BundlesHandler) handleDeleteBundle(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("delete bundle: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	if bundle.State != models.BundleStateDraft {
		writeError(w, http.StatusConflict, "conflict", "can only delete draft bundles")
		return
	}

	if err := h.db.DeleteReleaseBundle(ctx, bundle.ID); err != nil {
		h.log.Errorw("delete bundle: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

// ============================================================================
// Bundle Item Handlers
// ============================================================================

//nolint:dupl // handler boilerplate: identical structure, different entities
func (h *BundlesHandler) handleListItems(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("list items: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	items, err := h.db.ListBundleItems(ctx, bundle.ID)
	if err != nil {
		h.log.Errorw("list items: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if items == nil {
		items = []*models.ReleaseBundleItem{}
	}

	var result []map[string]any
	for _, item := range items {
		result = append(result, itemToMap(item))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": result,
	})
}

func (h *BundlesHandler) handleAddItems(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("add items: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	if bundle.State != models.BundleStateDraft {
		writeError(w, http.StatusConflict, "conflict", "can only add items to draft bundles")
		return
	}

	var body struct {
		Items []struct {
			SourceRepoID string  `json:"source_repo_id"`
			ComponentID  *string `json:"component_id"`
			SHA256       string  `json:"sha256"`
			ArtifactPath string  `json:"artifact_path"`
			Format       string  `json:"format"`
			SizeBytes    int64   `json:"size_bytes"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	var result []map[string]any

	for _, itemData := range body.Items {
		item := &models.ReleaseBundleItem{
			BundleID:     bundle.ID,
			SourceRepoID: itemData.SourceRepoID,
			ComponentID:  itemData.ComponentID,
			SHA256:       itemData.SHA256,
			ArtifactPath: itemData.ArtifactPath,
			Format:       itemData.Format,
			SizeBytes:    itemData.SizeBytes,
			AddedBy:      p.Username,
			AddedAt:      time.Now(),
		}

		if err := h.db.AddBundleItem(ctx, item); err != nil {
			h.log.Errorw("add items: db error adding item", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}

		result = append(result, itemToMap(item))
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"items": result,
	})
}

func (h *BundlesHandler) handleRemoveItem(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	sha256 := urlparams.Vars(r)["sha256"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("remove item: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	if bundle.State != models.BundleStateDraft {
		writeError(w, http.StatusConflict, "conflict", "can only remove items from draft bundles")
		return
	}

	item, err := h.db.GetBundleItem(ctx, bundle.ID, sha256)
	if err != nil {
		h.log.Errorw("remove item: db error getting item", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if item == nil {
		writeError(w, http.StatusNotFound, "not_found", "item not found")
		return
	}

	if err := h.db.RemoveBundleItem(ctx, bundle.ID, sha256); err != nil {
		h.log.Errorw("remove item: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

// ============================================================================
// Bundle Lifecycle Handlers
// ============================================================================

func (h *BundlesHandler) handleSeal(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("seal bundle: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	if bundle.State != models.BundleStateDraft {
		writeError(w, http.StatusConflict, "conflict", "can only seal draft bundles")
		return
	}

	// Check if bundle has items
	items, err := h.db.ListBundleItems(ctx, bundle.ID)
	if err != nil {
		h.log.Errorw("seal bundle: db error listing items", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if len(items) == 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "bundle must have at least one item")
		return
	}

	var body struct {
		GPGKeyID *string `json:"gpg_key_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		// Body is optional, so ignore decode errors
	}

	// GPG signing is not yet implemented
	if body.GPGKeyID != nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "GPG signing is not yet implemented")
		return
	}

	now := time.Now()
	bundle.State = models.BundleStateSealed
	bundle.SealedAt = &now
	bundle.SealedBy = &p.Username
	bundle.UpdatedAt = now

	if err := h.db.UpdateReleaseBundle(ctx, bundle); err != nil {
		h.log.Errorw("seal bundle: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, bundleToMap(bundle))
}

func (h *BundlesHandler) handlePromote(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("promote bundle: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	if bundle.State != models.BundleStateSealed {
		writeError(w, http.StatusConflict, "conflict", "can only promote sealed bundles")
		return
	}

	var body struct {
		ToStage    string `json:"to_stage"`
		TargetRepo string `json:"target_repo"`
		Notes      string `json:"notes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.ToStage == "" || body.TargetRepo == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "to_stage and target_repo are required")
		return
	}

	// Look up target repo by name
	targetRepo, err := h.db.GetRepositoryByName(ctx, body.TargetRepo)
	if err != nil {
		h.log.Errorw("promote bundle: db error getting target repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if targetRepo == nil {
		writeError(w, http.StatusNotFound, "not_found", "target repository not found")
		return
	}

	// Count items
	items, err := h.db.ListBundleItems(ctx, bundle.ID)
	if err != nil {
		h.log.Errorw("promote bundle: db error listing items", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Create promotion record
	now := time.Now()
	promotion := &models.BundlePromotion{
		BundleID:     bundle.ID,
		FromStage:    bundle.CurrentStage,
		ToStage:      body.ToStage,
		TargetRepoID: targetRepo.ID,
		Status:       models.PromotionStatusInProgress,
		PromotedBy:   p.Username,
		Notes:        body.Notes,
		ItemCount:    len(items),
		StartedAt:    now,
	}

	if err := h.db.CreateBundlePromotion(ctx, promotion); err != nil {
		h.log.Errorw("promote bundle: db error creating promotion", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Update bundle current stage
	bundle.CurrentStage = &body.ToStage
	bundle.UpdatedAt = now

	if err := h.db.UpdateReleaseBundle(ctx, bundle); err != nil {
		h.log.Errorw("promote bundle: db error updating bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Update promotion status to success
	promotion.Status = models.PromotionStatusSuccess
	promotion.CompletedAt = &now

	if err := h.db.UpdateBundlePromotion(ctx, promotion); err != nil {
		h.log.Errorw("promote bundle: db error updating promotion", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, promotionToMap(promotion))
}

func (h *BundlesHandler) handleArchive(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("archive bundle: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	if bundle.State != models.BundleStateSealed {
		writeError(w, http.StatusConflict, "conflict", "can only archive sealed bundles")
		return
	}

	now := time.Now()
	bundle.State = models.BundleStateArchived
	bundle.ArchivedAt = &now
	bundle.ArchivedBy = &p.Username
	bundle.UpdatedAt = now

	if err := h.db.UpdateReleaseBundle(ctx, bundle); err != nil {
		h.log.Errorw("archive bundle: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, bundleToMap(bundle))
}

// ============================================================================
// Promotion History Handler
// ============================================================================

//nolint:dupl // handler boilerplate: identical structure, different entities
func (h *BundlesHandler) handleListPromotions(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	version := urlparams.Vars(r)["version"]
	ctx := r.Context()

	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(ctx, p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	bundle, err := h.db.GetReleaseBundle(ctx, name, version)
	if err != nil {
		h.log.Errorw("list promotions: db error getting bundle", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if bundle == nil {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}

	promotions, err := h.db.ListBundlePromotions(ctx, bundle.ID)
	if err != nil {
		h.log.Errorw("list promotions: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if promotions == nil {
		promotions = []*models.BundlePromotion{}
	}

	var result []map[string]any
	for _, promotion := range promotions {
		result = append(result, promotionToMap(promotion))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"promotions": result,
	})
}

// ============================================================================
// Response Mapping Helpers
// ============================================================================

func bundleToMap(b *models.ReleaseBundle) map[string]any {
	return map[string]any{
		"id":            b.ID,
		"name":          b.Name,
		"version":       b.Version,
		"description":   b.Description,
		"state":         b.State,
		"pipeline_id":   b.PipelineID,
		"current_stage": b.CurrentStage,
		"created_by":    b.CreatedBy,
		"sealed_at":     formatTimePtr(b.SealedAt),
		"sealed_by":     b.SealedBy,
		"archived_at":   formatTimePtr(b.ArchivedAt),
		"archived_by":   b.ArchivedBy,
		"created_at":    formatTime(b.CreatedAt),
		"updated_at":    formatTime(b.UpdatedAt),
	}
}

func itemToMap(i *models.ReleaseBundleItem) map[string]any {
	return map[string]any{
		"id":             i.ID,
		"bundle_id":      i.BundleID,
		"source_repo_id": i.SourceRepoID,
		"component_id":   i.ComponentID,
		"sha256":         i.SHA256,
		"artifact_path":  i.ArtifactPath,
		"format":         i.Format,
		"size_bytes":     i.SizeBytes,
		"added_by":       i.AddedBy,
		"added_at":       formatTime(i.AddedAt),
	}
}

func promotionToMap(p *models.BundlePromotion) map[string]any {
	return map[string]any{
		"id":             p.ID,
		"bundle_id":      p.BundleID,
		"from_stage":     p.FromStage,
		"to_stage":       p.ToStage,
		"target_repo_id": p.TargetRepoID,
		"status":         p.Status,
		"promoted_by":    p.PromotedBy,
		"notes":          p.Notes,
		"error_message":  p.ErrorMessage,
		"item_count":     p.ItemCount,
		"started_at":     formatTime(p.StartedAt),
		"completed_at":   formatTimePtr(p.CompletedAt),
	}
}

func pipelineToMap(p *models.PromotionPipeline) map[string]any {
	// Unmarshal stages if needed
	if len(p.Stages) == 0 && len(p.StagesRaw) > 0 {
		_ = json.Unmarshal(p.StagesRaw, &p.Stages)
	}

	return map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"stages":      p.Stages,
		"created_by":  p.CreatedBy,
		"created_at":  formatTime(p.CreatedAt),
		"updated_at":  formatTime(p.UpdatedAt),
	}
}

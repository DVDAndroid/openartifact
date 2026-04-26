// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

var orgNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

type OrgsHandler struct {
	db  database.Database
	log *logger.Logger
}

func NewOrgsHandler(db database.Database, log *logger.Logger) *OrgsHandler {
	return &OrgsHandler{db: db, log: log}
}

func (h *OrgsHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/v1/orgs", h.handleCreate)
	r.Get("/api/v1/orgs", h.handleList)
	r.Get("/api/v1/orgs/{org}", h.handleGet)
	r.Put("/api/v1/orgs/{org}", h.handleUpdate)
	r.Delete("/api/v1/orgs/{org}", h.handleDelete)
	r.Get("/api/v1/orgs/{org}/members", h.handleListMembers)
	r.Post("/api/v1/orgs/{org}/members", h.handleAddMember)
	r.Put("/api/v1/orgs/{org}/members/{user_id}", h.handleUpdateMember)
	r.Delete("/api/v1/orgs/{org}/members/{user_id}", h.handleRemoveMember)
}

func (h *OrgsHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	// Validate name
	if !orgNameRe.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid_name", "organization name must be 1-100 alphanumeric characters, hyphens, or underscores")
		return
	}

	// Check if org already exists
	existing, err := h.db.GetOrganizationByName(r.Context(), req.Name)
	if err != nil {
		h.log.Errorw("check organization exists: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if existing != nil {
		writeError(w, http.StatusConflict, "already_exists", "organization name already taken")
		return
	}

	org := &models.Organization{
		Name:        req.Name,
		DisplayName: &req.DisplayName,
		Description: &req.Description,
		OwnerID:     p.UserID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.db.CreateOrganization(r.Context(), org); err != nil {
		h.log.Errorw("create organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Add creator as owner
	if err := h.db.AddOrgMember(r.Context(), org.ID, *p.UserID, "owner"); err != nil {
		h.log.Errorw("add org owner member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, orgToMap(org))
}

func (h *OrgsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgs, err := h.db.ListOrganizations(r.Context(), *p.UserID)
	if err != nil {
		h.log.Errorw("list organizations: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if orgs == nil {
		orgs = []*models.Organization{}
	}

	var result []responses.OrganizationResponse
	for _, org := range orgs {
		result = append(result, orgToMap(org))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"organizations": result,
	})
}

func (h *OrgsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgName := urlparams.Vars(r)["org"]

	org, err := h.db.GetOrganizationByName(r.Context(), orgName)
	if err != nil {
		h.log.Errorw("get organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if org == nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}

	// Check if user is member
	member, err := h.db.GetOrgMember(r.Context(), org.ID, *p.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if member == nil && (org.OwnerID == nil || *org.OwnerID != *p.UserID) {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return
	}

	writeJSON(w, http.StatusOK, orgToMap(org))
}

func (h *OrgsHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgName := urlparams.Vars(r)["org"]

	org, err := h.db.GetOrganizationByName(r.Context(), orgName)
	if err != nil {
		h.log.Errorw("get organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if org == nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}

	// Check permissions: owner or admin
	member, err := h.db.GetOrgMember(r.Context(), org.ID, *p.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	isOwner := org.OwnerID != nil && *org.OwnerID == *p.UserID
	isAdmin := member != nil && (member.Role == "owner" || member.Role == "admin")

	if !isOwner && !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	org.DisplayName = &req.DisplayName
	org.Description = &req.Description
	org.UpdatedAt = time.Now()

	if err := h.db.UpdateOrganization(r.Context(), org); err != nil {
		h.log.Errorw("update organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, orgToMap(org))
}

func (h *OrgsHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgName := urlparams.Vars(r)["org"]

	org, err := h.db.GetOrganizationByName(r.Context(), orgName)
	if err != nil {
		h.log.Errorw("get organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if org == nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}

	// Check permissions: owner only
	isOwner := org.OwnerID != nil && *org.OwnerID == *p.UserID
	if !isOwner {
		writeError(w, http.StatusForbidden, "forbidden", "only organization owner can delete")
		return
	}

	if err := h.db.DeleteOrganization(r.Context(), org.ID); err != nil {
		h.log.Errorw("delete organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func (h *OrgsHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgName := urlparams.Vars(r)["org"]

	org, err := h.db.GetOrganizationByName(r.Context(), orgName)
	if err != nil {
		h.log.Errorw("get organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if org == nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}

	// Check if user is member
	member, err := h.db.GetOrgMember(r.Context(), org.ID, *p.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if member == nil && (org.OwnerID == nil || *org.OwnerID != *p.UserID) {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return
	}

	members, err := h.db.ListOrgMembers(r.Context(), org.ID)
	if err != nil {
		h.log.Errorw("list organization members: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if members == nil {
		members = []*models.OrgMember{}
	}

	var result []responses.OrgMemberResponse
	for _, m := range members {
		result = append(result, memberToMap(m))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"members": result,
	})
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *OrgsHandler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgName := urlparams.Vars(r)["org"]

	org, err := h.db.GetOrganizationByName(r.Context(), orgName)
	if err != nil {
		h.log.Errorw("get organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if org == nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}

	// Check permissions: owner or admin
	member, err := h.db.GetOrgMember(r.Context(), org.ID, *p.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	isOwner := org.OwnerID != nil && *org.OwnerID == *p.UserID
	isAdmin := member != nil && (member.Role == "owner" || member.Role == "admin")

	if !isOwner && !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if req.Role == "" {
		req.Role = "member"
	}

	// Validate role
	if req.Role != "owner" && req.Role != "admin" && req.Role != "member" {
		writeError(w, http.StatusBadRequest, "invalid_role", "role must be owner, admin, or member")
		return
	}

	// Check if user exists
	user, err := h.db.GetUserByID(r.Context(), req.UserID)
	if err != nil {
		h.log.Errorw("get user: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if user == nil {
		writeError(w, http.StatusNotFound, "user_not_found", "user not found")
		return
	}

	// Check if already member
	existing, err := h.db.GetOrgMember(r.Context(), org.ID, req.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if existing != nil {
		writeError(w, http.StatusConflict, "already_member", "user is already a member")
		return
	}

	if err := h.db.AddOrgMember(r.Context(), org.ID, req.UserID, req.Role); err != nil {
		h.log.Errorw("add org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	newMember, err := h.db.GetOrgMember(r.Context(), org.ID, req.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, memberToMap(newMember))
}

func (h *OrgsHandler) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgName := urlparams.Vars(r)["org"]
	userID := urlparams.Vars(r)["user_id"]

	org, err := h.db.GetOrganizationByName(r.Context(), orgName)
	if err != nil {
		h.log.Errorw("get organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if org == nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}

	// Check permissions: owner or admin
	member, err := h.db.GetOrgMember(r.Context(), org.ID, *p.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	isOwner := org.OwnerID != nil && *org.OwnerID == *p.UserID
	isAdmin := member != nil && (member.Role == "owner" || member.Role == "admin")

	if !isOwner && !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	// Validate role
	if req.Role != "owner" && req.Role != "admin" && req.Role != "member" {
		writeError(w, http.StatusBadRequest, "invalid_role", "role must be owner, admin, or member")
		return
	}

	// Check if member exists
	targetMember, err := h.db.GetOrgMember(r.Context(), org.ID, userID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if targetMember == nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found")
		return
	}

	if err := h.db.UpdateOrgMemberRole(r.Context(), org.ID, userID, req.Role); err != nil {
		h.log.Errorw("update org member role: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	updatedMember, err := h.db.GetOrgMember(r.Context(), org.ID, userID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, memberToMap(updatedMember))
}

func (h *OrgsHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	orgName := urlparams.Vars(r)["org"]
	userID := urlparams.Vars(r)["user_id"]

	org, err := h.db.GetOrganizationByName(r.Context(), orgName)
	if err != nil {
		h.log.Errorw("get organization: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if org == nil {
		writeError(w, http.StatusNotFound, "not_found", "organization not found")
		return
	}

	// Check permissions: owner or admin
	member, err := h.db.GetOrgMember(r.Context(), org.ID, *p.UserID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	isOwner := org.OwnerID != nil && *org.OwnerID == *p.UserID
	isAdmin := member != nil && (member.Role == "owner" || member.Role == "admin")

	if !isOwner && !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return
	}

	// Check if member exists
	targetMember, err := h.db.GetOrgMember(r.Context(), org.ID, userID)
	if err != nil {
		h.log.Errorw("get org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if targetMember == nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found")
		return
	}

	if err := h.db.RemoveOrgMember(r.Context(), org.ID, userID); err != nil {
		h.log.Errorw("remove org member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

// Helper functions

func orgToMap(org *models.Organization) responses.OrganizationResponse {
	return responses.OrganizationResponse{
		ID:          org.ID,
		Name:        org.Name,
		DisplayName: org.DisplayName,
		Description: org.Description,
		OwnerID:     org.OwnerID,
		CreatedAt:   formatTime(org.CreatedAt),
		UpdatedAt:   formatTime(org.UpdatedAt),
	}
}

func memberToMap(member *models.OrgMember) responses.OrgMemberResponse {
	return responses.OrgMemberResponse{
		ID:        member.ID,
		OrgID:     member.OrgID,
		UserID:    member.UserID,
		Role:      member.Role,
		CreatedAt: formatTime(member.CreatedAt),
	}
}

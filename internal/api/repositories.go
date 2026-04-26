// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

var repoNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

//nolint:gochecknoglobals // immutable lookup table
var validFormats = map[string]bool{
	"deb":      true,
	"rpm":      true,
	"maven":    true,
	"npm":      true,
	"docker":   true,
	"helm":     true,
	"pypi":     true,
	"nuget":    true,
	"gomod":    true,
	"cargo":    true,
	"rubygems": true,
	"raw":      true,
	"conda":    true,
	"conan":    true,
	"ivy":      true,
	"sbt":      true,
}

type RepositoriesHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

func NewRepositoriesHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *RepositoriesHandler {
	return &RepositoriesHandler{db: db, log: log, evaluator: evaluator}
}

func (h *RepositoriesHandler) RegisterAuthed(r chi.Router) {
	r.Get("/api/v1/repositories", h.handleList)
	r.Post("/api/v1/repositories", h.handleCreate)
	r.Get("/api/v1/repository/{name}", h.handleGet)
	r.Patch("/api/v1/repository/{name}", h.handleUpdate)
	r.Delete("/api/v1/repository/{name}", h.handleDelete)
	r.Get("/api/v1/repositories/{name}/packages", h.handlePackages)
	r.Get("/api/v1/repositories/{name}/members", h.handleGetMembers)
	r.Put("/api/v1/repositories/{name}/members", h.handleUpdateMembers)
}

func (h *RepositoriesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	formatFilter := q.Get("format")
	page := parseIntMin(q.Get("page"), 1, 1)
	limit := parseIntClamp(q.Get("limit"), 25, 1, 100)

	ctx := r.Context()
	p := currentPrincipal(r)

	var (
		repos []*models.Repository
		total int
	)

	if p != nil && !p.IsAnonymous {
		for dbPage := 1; ; dbPage++ {
			batch, err := h.db.ListRepositories(ctx, dbPage, 100, formatFilter)
			if err != nil {
				h.log.Errorw("list repositories: db error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "database error")

				return
			}

			for _, repo := range batch {
				ok, err := h.evaluator.Evaluate(ctx, *p, repo.Name, "", "read")
				if err == nil && ok {
					repos = append(repos, repo)
				}
			}

			if len(batch) < 100 {
				break
			}
		}

		total = len(repos)
		start := (page - 1) * limit
		if start >= total {
			repos = nil
		} else {
			end := min(start+limit, total)
			repos = repos[start:end]
		}
	} else {
		var err error
		repos, err = h.db.ListRepositories(ctx, page, limit, formatFilter)
		if err != nil {
			h.log.Errorw("list repositories: db error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}

		total, err = h.db.CountRepositories(ctx, formatFilter)
		if err != nil {
			h.log.Errorw("count repositories: db error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}
	}

	filtered := make([]responses.RepositoryResponse, 0, len(repos))

	for _, repo := range repos {
		filtered = append(filtered, repoToMap(repo))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":        total,
		"page":         page,
		"limit":        limit,
		"repositories": filtered,
	})
}

func (h *RepositoriesHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("get repository: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)
	if p != nil && !p.IsAnonymous {
		ok, err := h.evaluator.Evaluate(ctx, *p, repo.Name, "", "read")
		if err != nil || !ok {
			writeError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
	}

	writeJSON(w, http.StatusOK, repoToMap(repo))
}

func (h *RepositoriesHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	var body struct {
		Name                    string   `json:"name"`
		Format                  string   `json:"format"`
		Description             string   `json:"description"`
		Public                  bool     `json:"public"`
		RepoType                string   `json:"repo_type"`
		UpstreamURL             string   `json:"upstream_url"`
		UpstreamUsername        string   `json:"upstream_username"`
		UpstreamPassword        string   `json:"upstream_password"`
		CacheTTLSeconds         int      `json:"cache_ttl_seconds"`
		NegativeCacheTTLSeconds int      `json:"negative_cache_ttl_seconds"`
		MemberRepos             []string `json:"member_repos"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if !repoNameRe.MatchString(body.Name) {
		writeError(w, http.StatusBadRequest, "validation_failed", "name must be 1-100 alphanumeric/hyphen/underscore characters")
		return
	}

	if !validFormats[body.Format] {
		writeError(w, http.StatusBadRequest, "validation_failed", "format must be one of: deb, rpm, maven, npm, docker, helm")
		return
	}

	// Validate repo_type
	repoType := body.RepoType
	if repoType == "" {
		repoType = "hosted"
	}

	if repoType != "hosted" && repoType != "proxy" && repoType != "group" {
		writeError(w, http.StatusBadRequest, "validation_failed", "repo_type must be one of: hosted, proxy, group")
		return
	}

	// Validate proxy-specific fields
	if repoType == "proxy" {
		if body.UpstreamURL == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "upstream_url is required for proxy repositories")
			return
		}

		if body.CacheTTLSeconds == 0 {
			body.CacheTTLSeconds = 3600
		}

		if body.NegativeCacheTTLSeconds == 0 {
			body.NegativeCacheTTLSeconds = 60
		}
	}

	now := time.Now()
	systemOrgID := "00000000-0000-0000-0000-000000000001"

	repo := &models.Repository{
		Name:                    body.Name,
		Format:                  body.Format,
		Description:             body.Description,
		Public:                  body.Public,
		RepoType:                repoType,
		UpstreamURL:             body.UpstreamURL,
		UpstreamUsername:        body.UpstreamUsername,
		UpstreamPasswordEnc:     body.UpstreamPassword,
		OrgID:                   &systemOrgID,
		CacheTTLSeconds:         body.CacheTTLSeconds,
		NegativeCacheTTLSeconds: body.NegativeCacheTTLSeconds,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := h.db.CreateRepository(r.Context(), repo); err != nil {
		h.log.Errorw("create repository: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Add group members if this is a group repo
	if repoType == "group" {
		ctx := r.Context()
		for priority, memberName := range body.MemberRepos {
			memberRepo, err := h.db.GetRepositoryByName(ctx, memberName)
			if err != nil {
				h.log.Errorw("failed to get member repo", "name", memberName, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "database error")

				return
			}

			if memberRepo == nil {
				writeError(w, http.StatusBadRequest, "validation_failed", "member repository not found: "+memberName)
				return
			}

			if err := h.db.AddGroupMember(ctx, repo.ID, memberRepo.ID, priority); err != nil {
				h.log.Errorw("failed to add group member", "group_id", repo.ID, "member_id", memberRepo.ID, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "database error")

				return
			}
		}
	}

	writeJSON(w, http.StatusCreated, repoToMap(repo))
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (h *RepositoriesHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("update repository: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	var body struct {
		Description             *string  `json:"description"`
		Public                  *bool    `json:"public"`
		UpstreamURL             *string  `json:"upstream_url"`
		UpstreamUsername        *string  `json:"upstream_username"`
		UpstreamPassword        *string  `json:"upstream_password"`
		CacheTTLSeconds         *int     `json:"cache_ttl_seconds"`
		NegativeCacheTTLSeconds *int     `json:"negative_cache_ttl_seconds"`
		Blocked                 *bool    `json:"blocked"`
		MemberRepos             []string `json:"member_repos"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if body.Description != nil {
		repo.Description = *body.Description
	}

	if body.Public != nil {
		repo.Public = *body.Public
	}

	if body.UpstreamURL != nil {
		repo.UpstreamURL = *body.UpstreamURL
	}

	if body.UpstreamUsername != nil {
		repo.UpstreamUsername = *body.UpstreamUsername
	}

	if body.UpstreamPassword != nil {
		repo.UpstreamPasswordEnc = *body.UpstreamPassword
	}

	if body.CacheTTLSeconds != nil {
		repo.CacheTTLSeconds = *body.CacheTTLSeconds
	}

	if body.NegativeCacheTTLSeconds != nil {
		repo.NegativeCacheTTLSeconds = *body.NegativeCacheTTLSeconds
	}

	if body.Blocked != nil {
		repo.Blocked = *body.Blocked
	}

	if err := h.db.UpdateRepository(ctx, repo); err != nil {
		h.log.Errorw("update repository: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	// Update group members if provided
	if len(body.MemberRepos) > 0 && repo.RepoType == "group" {
		// Remove all existing members
		existingMembers, err := h.db.ListGroupMembers(ctx, repo.ID)
		if err != nil {
			h.log.Errorw("failed to list group members", "repo_id", repo.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}

		for _, member := range existingMembers {
			if err := h.db.RemoveGroupMember(ctx, repo.ID, member.MemberRepoID); err != nil {
				h.log.Errorw("failed to remove group member", "group_id", repo.ID, "member_id", member.MemberRepoID, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "database error")

				return
			}
		}

		// Add new members
		for priority, memberName := range body.MemberRepos {
			memberRepo, err := h.db.GetRepositoryByName(ctx, memberName)
			if err != nil {
				h.log.Errorw("failed to get member repo", "name", memberName, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "database error")

				return
			}

			if memberRepo == nil {
				writeError(w, http.StatusBadRequest, "validation_failed", "member repository not found: "+memberName)
				return
			}

			if err := h.db.AddGroupMember(ctx, repo.ID, memberRepo.ID, priority); err != nil {
				h.log.Errorw("failed to add group member", "group_id", repo.ID, "member_id", memberRepo.ID, "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "database error")

				return
			}
		}
	}

	writeJSON(w, http.StatusOK, repoToMap(repo))
}

func (h *RepositoriesHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("delete repository: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	if err := h.db.DeleteRepository(ctx, repo.ID); err != nil {
		h.log.Errorw("delete repository: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func (h *RepositoriesHandler) handlePackages(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	q := r.URL.Query()
	formatFilter := q.Get("format")
	page := parseIntMin(q.Get("page"), 1, 1)
	limit := parseIntClamp(q.Get("limit"), 25, 1, 100)

	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("packages: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	p := currentPrincipal(r)
	if p != nil && !p.IsAnonymous {
		ok, err := h.evaluator.Evaluate(ctx, *p, repo.Name, "", "read")
		if err != nil || !ok {
			writeError(w, http.StatusForbidden, "forbidden", "access denied")
			return
		}
	}

	components, total, err := h.db.ListComponentsByRepo(ctx, repo.ID, formatFilter, page, limit)
	if err != nil {
		h.log.Errorw("packages: db error listing components", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	packages := make([]map[string]any, 0, len(components))
	for _, c := range components {
		packages = append(packages, map[string]any{
			"id":         c.ID,
			"name":       c.Name,
			"version":    c.Version,
			"format":     c.Format,
			"arch":       c.Arch,
			"size":       c.PackageSize,
			"filename":   c.Filename,
			"created_at": c.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":    total,
		"page":     page,
		"limit":    limit,
		"packages": packages,
	})
}

func (h *RepositoriesHandler) handleGetMembers(w http.ResponseWriter, r *http.Request) {
	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("get members: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	if repo.RepoType != "group" {
		writeError(w, http.StatusBadRequest, "bad_request", "repository is not a group")
		return
	}

	members, err := h.db.ListGroupMembers(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("get members: db error listing members", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	var memberList []map[string]any

	for _, member := range members {
		memberList = append(memberList, map[string]any{
			"id":       member.MemberRepoID,
			"priority": member.Priority,
		})
	}

	if memberList == nil {
		memberList = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"members": memberList,
	})
}

func (h *RepositoriesHandler) handleUpdateMembers(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)

	isAdmin, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil || !isAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin access required")
		return
	}

	name := urlparams.Vars(r)["name"]
	ctx := r.Context()

	repo, err := h.db.GetRepositoryByName(ctx, name)
	if err != nil {
		h.log.Errorw("update members: db error getting repo", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if repo == nil {
		writeError(w, http.StatusNotFound, "not_found", "repository not found")
		return
	}

	if repo.RepoType != "group" {
		writeError(w, http.StatusBadRequest, "bad_request", "repository is not a group")
		return
	}

	var body struct {
		MemberRepos []string `json:"member_repos"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	// Remove all existing members
	existingMembers, err := h.db.ListGroupMembers(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("failed to list group members", "repo_id", repo.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	for _, member := range existingMembers {
		if err := h.db.RemoveGroupMember(ctx, repo.ID, member.MemberRepoID); err != nil {
			h.log.Errorw("failed to remove group member", "group_id", repo.ID, "member_id", member.MemberRepoID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}
	}

	// Add new members
	for priority, memberName := range body.MemberRepos {
		memberRepo, err := h.db.GetRepositoryByName(ctx, memberName)
		if err != nil {
			h.log.Errorw("failed to get member repo", "name", memberName, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}

		if memberRepo == nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "member repository not found: "+memberName)
			return
		}

		if err := h.db.AddGroupMember(ctx, repo.ID, memberRepo.ID, priority); err != nil {
			h.log.Errorw("failed to add group member", "group_id", repo.ID, "member_id", memberRepo.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return
		}
	}

	// Return updated members
	members, err := h.db.ListGroupMembers(ctx, repo.ID)
	if err != nil {
		h.log.Errorw("update members: db error listing members", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	var memberList []map[string]any
	for _, member := range members {
		memberList = append(memberList, map[string]any{
			"id":       member.MemberRepoID,
			"priority": member.Priority,
		})
	}

	if memberList == nil {
		memberList = []map[string]any{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"members": memberList,
	})
}

func repoToMap(r *models.Repository) responses.RepositoryResponse {
	desc := r.Description // copy to allow taking address

	return responses.RepositoryResponse{
		ID:          r.ID,
		Name:        r.Name,
		Format:      r.Format,
		Description: &desc,
		Public:      r.Public,
		RepoType:    r.RepoType,
		CreatedAt:   formatTime(r.CreatedAt),
		UpdatedAt:   formatTime(r.UpdatedAt),
	}
}

func parseIntMin(s string, defaultVal, minVal int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v < minVal {
		if s == "" {
			return defaultVal
		}

		return minVal
	}

	return v
}

func parseIntClamp(s string, defaultVal, minVal, maxVal int) int { //nolint:unparam // minVal kept for API clarity and future flexibility
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}

	if v < minVal {
		return minVal
	}

	if v > maxVal {
		return maxVal
	}

	return v
}

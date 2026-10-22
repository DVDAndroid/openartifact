// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"math"
	"net/http"
	"regexp"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/logger"
)

var (
	projectNameRe   = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)
	projectPrefixRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{0,100}$`)
)

const (
	projectRoleMessage      = "role must be owner, maintainer, developer, or viewer"
	projectGroupRoleMessage = "role must be maintainer, developer, or viewer"
)

// ProjectsHandler serves /api/v1/projects: projects, their members and the
// groups attached to them. The RBAC middleware currently gates every route on
// the global "manage" action; the project-role checks below are a second layer
// that also holds when a route is reached by a project member.
type ProjectsHandler struct {
	db        database.Database
	log       *logger.Logger
	evaluator *auth.Evaluator
}

// NewProjectsHandler returns the projects handler. evaluator decides who is a
// global admin (implicit owner of every project); nil means nobody is.
func NewProjectsHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator) *ProjectsHandler {
	return &ProjectsHandler{db: db, log: log, evaluator: evaluator}
}

func (h *ProjectsHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/v1/projects", h.handleCreate)
	r.Get("/api/v1/projects", h.handleList)
	r.Get("/api/v1/projects/{project}", h.handleGet)
	r.Put("/api/v1/projects/{project}", h.handleUpdate)
	r.Delete("/api/v1/projects/{project}", h.handleDelete)
	r.Get("/api/v1/projects/{project}/members", h.handleListMembers)
	r.Post("/api/v1/projects/{project}/members", h.handleAddMember)
	r.Put("/api/v1/projects/{project}/members/{user_id}", h.handleUpdateMember)
	r.Delete("/api/v1/projects/{project}/members/{user_id}", h.handleRemoveMember)
	r.Get("/api/v1/projects/{project}/groups", h.handleListGroups)
	r.Put("/api/v1/projects/{project}/groups/{group_id}", h.handleSetGroup)
	r.Delete("/api/v1/projects/{project}/groups/{group_id}", h.handleRemoveGroup)
}

func validProjectRole(role string) bool {
	return models.ProjectRoleRank(role) > 0
}

func validProjectGroupRole(role string) bool {
	return validProjectRole(role) && role != models.ProjectRoleOwner
}

// projectPolicy is the policy part of the create/update bodies. Pointer fields
// leave the stored value alone when omitted; allowed_formats [] means every
// format (stored NULL).
type projectPolicy struct {
	AllowedFormats   *[]string `json:"allowed_formats"`
	AllowedRepoTypes *[]string `json:"allowed_repo_types"`
	RepoNamePrefix   *string   `json:"repo_name_prefix"`
	AllowPublicRepos *bool     `json:"allow_public_repos"`
	MaxRepositories  *int      `json:"max_repositories"`
	TokenMaxTTLDays  *int      `json:"token_max_ttl_days"`
}

func (pp *projectPolicy) set() bool {
	return pp.AllowedFormats != nil || pp.AllowedRepoTypes != nil || pp.RepoNamePrefix != nil ||
		pp.AllowPublicRepos != nil || pp.MaxRepositories != nil || pp.TokenMaxTTLDays != nil
}

// validate returns the validation message for the first invalid field, or "".
func (pp *projectPolicy) validate() string {
	if pp.AllowedFormats != nil {
		for _, f := range *pp.AllowedFormats {
			if !validFormats[f] {
				return "allowed_formats contains an unknown format"
			}
		}
	}

	if pp.AllowedRepoTypes != nil {
		if len(*pp.AllowedRepoTypes) == 0 {
			return "allowed_repo_types must not be empty"
		}

		for _, t := range *pp.AllowedRepoTypes {
			if t != models.ProjectRepoTypeHosted && t != models.ProjectRepoTypeProxy && t != models.ProjectRepoTypeGroup {
				return "allowed_repo_types must contain hosted, proxy, or group"
			}
		}
	}

	if pp.RepoNamePrefix != nil && !projectPrefixRe.MatchString(*pp.RepoNamePrefix) {
		return "repo_name_prefix must be at most 100 alphanumeric characters, hyphens, or underscores"
	}

	for _, n := range []*int{pp.MaxRepositories, pp.TokenMaxTTLDays} {
		if n != nil && (*n < 0 || *n > math.MaxInt32) {
			return "max_repositories and token_max_ttl_days must be between 0 and 2147483647"
		}
	}

	return ""
}

// apply copies the set fields onto project, de-duplicating the lists.
func (pp *projectPolicy) apply(project *models.Project) {
	if pp.AllowedFormats != nil {
		project.AllowedFormats = nil

		if len(*pp.AllowedFormats) > 0 {
			project.AllowedFormats = dedupe(*pp.AllowedFormats)
		}
	}

	if pp.AllowedRepoTypes != nil {
		project.AllowedRepoTypes = dedupe(*pp.AllowedRepoTypes)
	}

	if pp.RepoNamePrefix != nil {
		project.RepoNamePrefix = *pp.RepoNamePrefix
	}

	if pp.AllowPublicRepos != nil {
		project.AllowPublicRepos = *pp.AllowPublicRepos
	}

	if pp.MaxRepositories != nil {
		project.MaxRepositories = *pp.MaxRepositories
	}

	if pp.TokenMaxTTLDays != nil {
		project.TokenMaxTTLDays = *pp.TokenMaxTTLDays
	}
}

func dedupe(in []string) []string {
	out := make([]string, 0, len(in))

	for _, s := range in {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}

	return out
}

// projectAccess describes what the calling user may do in a project.
type projectAccess struct {
	project *models.Project
	userID  string
	// admin: global admin, an implicit owner of every project.
	admin bool
	// role is the caller's membership role ("" for a non-member).
	role string
	// isRecordedOwner: the project's owner_id.
	isRecordedOwner bool
}

func (a *projectAccess) rank() int {
	switch {
	case a.admin || a.isRecordedOwner:
		return models.ProjectRoleRank(models.ProjectRoleOwner)
	default:
		return models.ProjectRoleRank(a.role)
	}
}

func (a *projectAccess) isMember() bool { return a.admin || a.rank() > 0 }

func (a *projectAccess) isOwner() bool {
	return a.rank() >= models.ProjectRoleRank(models.ProjectRoleOwner)
}

// isManager: maintainer or owner.
func (a *projectAccess) isManager() bool {
	return a.rank() >= models.ProjectRoleRank(models.ProjectRoleMaintainer)
}

// canGrant reports whether the caller may give (or take away) role: owners any
// role, maintainers only roles below maintainer.
func (a *projectAccess) canGrant(role string) bool {
	return a.isOwner() || (a.isManager() && models.ProjectRoleRank(role) < models.ProjectRoleRank(models.ProjectRoleMaintainer))
}

func (h *ProjectsHandler) isAdmin(r *http.Request, p *auth.Principal) bool {
	if h.evaluator == nil {
		return false
	}

	ok, err := principalIsAdmin(r.Context(), p, h.evaluator)
	if err != nil {
		h.log.Errorw("project admin check failed", "error", err)
		return false
	}

	return ok
}

// loadAccess authenticates the caller, loads the project named in the URL and
// resolves the caller's role. It writes the error response and returns false on
// failure.
func (h *ProjectsHandler) loadAccess(w http.ResponseWriter, r *http.Request) (*projectAccess, bool) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return nil, false
	}

	project, err := h.db.GetProjectByName(r.Context(), urlparams.Vars(r)["project"])
	if err != nil {
		h.log.Errorw("get project: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return nil, false
	}

	if project == nil {
		writeError(w, http.StatusNotFound, "not_found", "project not found")
		return nil, false
	}

	member, err := h.db.GetProjectMember(r.Context(), project.ID, *p.UserID)
	if err != nil {
		h.log.Errorw("get project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return nil, false
	}

	access := &projectAccess{project: project, userID: *p.UserID, admin: h.isAdmin(r, p)}
	access.isRecordedOwner = project.OwnerID != nil && *project.OwnerID == *p.UserID

	if member != nil {
		access.role = member.Role
	}

	return access, true
}

func (h *ProjectsHandler) requireMember(w http.ResponseWriter, r *http.Request) (*projectAccess, bool) {
	access, ok := h.loadAccess(w, r)
	if !ok {
		return nil, false
	}

	if !access.isMember() {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return nil, false
	}

	return access, true
}

func (h *ProjectsHandler) requireManager(w http.ResponseWriter, r *http.Request) (*projectAccess, bool) {
	access, ok := h.loadAccess(w, r)
	if !ok {
		return nil, false
	}

	if !access.isManager() {
		writeError(w, http.StatusForbidden, "forbidden", "access denied")
		return nil, false
	}

	return access, true
}

// countOwners returns the number of members holding the owner role.
func (h *ProjectsHandler) countOwners(r *http.Request, projectID string) (int, error) {
	members, err := h.db.ListProjectMembers(r.Context(), projectID)
	if err != nil {
		return 0, err //nolint:wrapcheck // logged by caller
	}

	n := 0

	for _, m := range members {
		if m.Role == models.ProjectRoleOwner {
			n++
		}
	}

	return n, nil
}

func (h *ProjectsHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		projectPolicy
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if !projectNameRe.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "validation_failed", "project name must be 1-100 alphanumeric characters, hyphens, or underscores")
		return
	}

	slug := models.ProjectSlug(req.Name)
	if slug == "" || len(slug) > models.MaxProjectSlugLen {
		writeError(w, http.StatusBadRequest, "validation_failed", "project name must yield a URL slug of 1-64 lowercase letters, digits and hyphens")
		return
	}

	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, "validation_failed", msg)
		return
	}

	now := time.Now()
	project := &models.Project{
		Name:             req.Name,
		DisplayName:      &req.DisplayName,
		Description:      &req.Description,
		OwnerID:          p.UserID,
		Slug:             slug,
		AllowedRepoTypes: []string{models.ProjectRepoTypeHosted},
		RepoNamePrefix:   slug + "-",
		MaxRepositories:  defaultProjectMaxRepositories,
		TokenMaxTTLDays:  defaultProjectTokenMaxTTLDays,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	req.apply(project)

	// The unique constraints are authoritative: no check-then-insert race.
	if err := h.db.CreateProject(r.Context(), project); err != nil {
		if database.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "conflict", "project name or slug already taken")
			return
		}

		h.log.Errorw("create project: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if err := h.db.AddProjectMember(r.Context(), project.ID, *p.UserID, models.ProjectRoleOwner); err != nil {
		h.log.Errorw("add project owner member: db error", "error", err)

		// Do not leave a project with no owner membership behind.
		if delErr := h.db.DeleteProject(r.Context(), project.ID); delErr != nil {
			h.log.Errorw("rollback project create failed", "project_id", project.ID, "error", delErr)
		}

		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, projectToResponse(project))
}

// Defaults of a new project's policy (the migration gives existing projects the
// same values).
const (
	defaultProjectMaxRepositories = 50
	defaultProjectTokenMaxTTLDays = 365
)

func (h *ProjectsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var (
		projects []*models.Project
		err      error
	)

	if r.URL.Query().Get("all") == "true" {
		if !h.isAdmin(r, p) {
			writeError(w, http.StatusForbidden, "forbidden", "admin access required")
			return
		}

		projects, err = h.db.ListAllProjects(r.Context())
	} else {
		projects, err = h.db.ListProjects(r.Context(), *p.UserID)
	}

	if err != nil {
		h.log.Errorw("list projects: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	result := make([]responses.ProjectResponse, 0, len(projects))
	for _, project := range projects {
		result = append(result, projectToResponse(project))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"projects": result,
	})
}

func (h *ProjectsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireMember(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, projectToResponse(access.project))
}

func (h *ProjectsHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireManager(w, r)
	if !ok {
		return
	}

	// Pointer fields: omitted fields keep their current value.
	var req struct {
		DisplayName *string `json:"display_name"`
		Description *string `json:"description"`
		projectPolicy
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, "validation_failed", msg)
		return
	}

	// Policy decides what self-service users may do: global admins only.
	if req.set() && !access.admin {
		writeError(w, http.StatusForbidden, "forbidden", "only a global admin can change the project policy")
		return
	}

	project := access.project

	if req.DisplayName != nil {
		project.DisplayName = req.DisplayName
	}

	if req.Description != nil {
		project.Description = req.Description
	}

	req.apply(project)

	project.UpdatedAt = time.Now()

	if err := h.db.UpdateProject(r.Context(), project); err != nil {
		h.log.Errorw("update project: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, projectToResponse(project))
}

func (h *ProjectsHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	access, ok := h.loadAccess(w, r)
	if !ok {
		return
	}

	// Only the project's recorded owner (or a global admin) may delete it.
	if !access.admin && !access.isRecordedOwner {
		writeError(w, http.StatusForbidden, "forbidden", "only the project owner can delete")
		return
	}

	if err := h.db.DeleteProject(r.Context(), access.project.ID); err != nil {
		if database.IsForeignKeyViolation(err) {
			writeError(w, http.StatusConflict, "conflict", "project still owns repositories (including repositories in the trash, which must be purged or restored first)")
			return
		}

		h.log.Errorw("delete project: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func (h *ProjectsHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireMember(w, r)
	if !ok {
		return
	}

	members, err := h.db.ListProjectMembers(r.Context(), access.project.ID)
	if err != nil {
		h.log.Errorw("list project members: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	result := make([]responses.ProjectMemberResponse, 0, len(members))
	for _, m := range members {
		result = append(result, memberToResponse(m))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"members": result,
	})
}

func (h *ProjectsHandler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireManager(w, r)
	if !ok {
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if req.Role == "" {
		req.Role = models.ProjectRoleViewer
	}

	if !validProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "validation_failed", projectRoleMessage)
		return
	}

	// Escalation guard: owners grant any role, maintainers only below maintainer.
	if !access.canGrant(req.Role) {
		writeError(w, http.StatusForbidden, "forbidden", "not allowed to grant this role")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), req.UserID)
	if err != nil {
		h.log.Errorw("get user: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if user == nil {
		writeError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	existing, err := h.db.GetProjectMember(r.Context(), access.project.ID, req.UserID)
	if err != nil {
		h.log.Errorw("get project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if existing != nil {
		writeError(w, http.StatusConflict, "conflict", "user is already a member")
		return
	}

	if err := h.db.AddProjectMember(r.Context(), access.project.ID, req.UserID, req.Role); err != nil {
		if database.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, "conflict", "user is already a member")
			return
		}

		h.log.Errorw("add project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	newMember, err := h.db.GetProjectMember(r.Context(), access.project.ID, req.UserID)
	if err != nil {
		h.log.Errorw("get project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusCreated, memberToResponse(newMember))
}

// guardMemberChange enforces the protection rules for changing or removing
// target (whose new role is newRole, or "" when removing). It writes the error
// response and returns false when the change is not allowed.
func (h *ProjectsHandler) guardMemberChange(w http.ResponseWriter, r *http.Request, access *projectAccess, target *models.ProjectMember, newRole string) bool {
	if newRole == target.Role {
		return true
	}

	if access.project.OwnerID != nil && *access.project.OwnerID == target.UserID {
		writeError(w, http.StatusConflict, "conflict", "the project owner cannot be demoted or removed")
		return false
	}

	// Touching a maintainer or owner, or granting one of those roles, takes an owner.
	if !access.canGrant(target.Role) || (newRole != "" && !access.canGrant(newRole)) {
		writeError(w, http.StatusForbidden, "forbidden", "not allowed to change this member")
		return false
	}

	if target.Role == models.ProjectRoleOwner {
		owners, err := h.countOwners(r, access.project.ID)
		if err != nil {
			h.log.Errorw("count project owners: db error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "database error")

			return false
		}

		if owners <= 1 {
			writeError(w, http.StatusConflict, "conflict", "cannot remove or demote the last owner")
			return false
		}
	}

	return true
}

func (h *ProjectsHandler) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireManager(w, r)
	if !ok {
		return
	}

	userID := urlparams.Vars(r)["user_id"]

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if !validProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "validation_failed", projectRoleMessage)
		return
	}

	targetMember, err := h.db.GetProjectMember(r.Context(), access.project.ID, userID)
	if err != nil {
		h.log.Errorw("get project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if targetMember == nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found")
		return
	}

	if !h.guardMemberChange(w, r, access, targetMember, req.Role) {
		return
	}

	if err := h.db.UpdateProjectMemberRole(r.Context(), access.project.ID, userID, req.Role); err != nil {
		h.log.Errorw("update project member role: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	updatedMember, err := h.db.GetProjectMember(r.Context(), access.project.ID, userID)
	if err != nil {
		h.log.Errorw("get project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, memberToResponse(updatedMember))
}

func (h *ProjectsHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireManager(w, r)
	if !ok {
		return
	}

	userID := urlparams.Vars(r)["user_id"]

	targetMember, err := h.db.GetProjectMember(r.Context(), access.project.ID, userID)
	if err != nil {
		h.log.Errorw("get project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if targetMember == nil {
		writeError(w, http.StatusNotFound, "not_found", "member not found")
		return
	}

	if !h.guardMemberChange(w, r, access, targetMember, "") {
		return
	}

	if err := h.db.RemoveProjectMember(r.Context(), access.project.ID, userID); err != nil {
		h.log.Errorw("remove project member: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

func (h *ProjectsHandler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireMember(w, r)
	if !ok {
		return
	}

	groups, err := h.db.ListProjectGroups(r.Context(), access.project.ID)
	if err != nil {
		h.log.Errorw("list project groups: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	result := make([]responses.ProjectGroupResponse, 0, len(groups))
	for _, g := range groups {
		result = append(result, projectGroupToResponse(g))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"groups": result,
	})
}

func (h *ProjectsHandler) handleSetGroup(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireManager(w, r)
	if !ok {
		return
	}

	groupID := urlparams.Vars(r)["group_id"]

	var req struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
		return
	}

	if !validProjectGroupRole(req.Role) {
		writeError(w, http.StatusBadRequest, "validation_failed", projectGroupRoleMessage)
		return
	}

	group, err := h.db.GetGroupByID(r.Context(), groupID)
	if err != nil {
		h.log.Errorw("get group: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if group == nil {
		writeError(w, http.StatusNotFound, "not_found", "group not found")
		return
	}

	if err := h.db.SetProjectGroup(r.Context(), access.project.ID, group.ID, req.Role, &access.userID); err != nil {
		h.log.Errorw("set project group: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	attached, err := h.db.GetProjectGroup(r.Context(), access.project.ID, group.ID)
	if err != nil || attached == nil {
		if err != nil {
			h.log.Errorw("get project group: db error", "error", err)
		}

		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeJSON(w, http.StatusOK, projectGroupToResponse(attached))
}

func (h *ProjectsHandler) handleRemoveGroup(w http.ResponseWriter, r *http.Request) {
	access, ok := h.requireManager(w, r)
	if !ok {
		return
	}

	groupID := urlparams.Vars(r)["group_id"]

	existing, err := h.db.GetProjectGroup(r.Context(), access.project.ID, groupID)
	if err != nil {
		h.log.Errorw("get project group: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if existing == nil {
		writeError(w, http.StatusNotFound, "not_found", "group is not attached to the project")
		return
	}

	if err := h.db.RemoveProjectGroup(r.Context(), access.project.ID, groupID); err != nil {
		h.log.Errorw("remove project group: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	writeNoContent(w)
}

// Helper functions

func projectToResponse(project *models.Project) responses.ProjectResponse {
	return responses.ProjectResponse{
		ID:               project.ID,
		Name:             project.Name,
		DisplayName:      project.DisplayName,
		Description:      project.Description,
		OwnerID:          project.OwnerID,
		Slug:             project.Slug,
		AllowedFormats:   project.AllowedFormats,
		AllowedRepoTypes: project.AllowedRepoTypes,
		RepoNamePrefix:   project.RepoNamePrefix,
		AllowPublicRepos: project.AllowPublicRepos,
		MaxRepositories:  project.MaxRepositories,
		TokenMaxTTLDays:  project.TokenMaxTTLDays,
		CreatedAt:        formatTime(project.CreatedAt),
		UpdatedAt:        formatTime(project.UpdatedAt),
	}
}

func memberToResponse(member *models.ProjectMember) responses.ProjectMemberResponse {
	return responses.ProjectMemberResponse{
		ID:        member.ID,
		ProjectID: member.ProjectID,
		UserID:    member.UserID,
		Role:      member.Role,
		CreatedAt: formatTime(member.CreatedAt),
	}
}

func projectGroupToResponse(g *models.ProjectGroup) responses.ProjectGroupResponse {
	return responses.ProjectGroupResponse{
		ProjectID: g.ProjectID,
		GroupID:   g.GroupID,
		GroupName: g.GroupName,
		Role:      g.Role,
		CreatedAt: formatTime(g.CreatedAt),
	}
}

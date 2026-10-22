// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/middleware"
)

const testAcmeID = "proj-acme"

// projectsMockDB is an in-memory project store for the projects and
// repositories handlers. admins holds the user IDs that are global admins.
type projectsMockDB struct {
	repoCreateMockDB

	projects     map[string]*models.Project // by name
	members      map[string]*models.ProjectMember
	users        map[string]*models.User
	groups       map[string]*models.Group
	projectGroup map[string]*models.ProjectGroup // by group ID, project acme only
	admins       map[string]bool
	userProjects map[string][]*models.Project

	createdProject *models.Project
	updated        *models.Project
	deleted        []string
	removed        []string
	roleUpdates    map[string]string
	setGroups      map[string]string
	setGroupBy     *string
	removedGroups  []string
	listedAll      bool

	repo      *models.Repository
	updRepo   *models.Repository
	deleteErr error
}

var _ database.Database = (*projectsMockDB)(nil)

func (db *projectsMockDB) GetPermissionTargetsForPrincipal(_ context.Context, userID string, _ []string) ([]*models.PermissionTarget, error) {
	if db.admins[userID] {
		return adminTargets(), nil
	}

	return nil, nil
}

func (db *projectsMockDB) GetProjectByName(_ context.Context, name string) (*models.Project, error) {
	return db.projects[name], nil
}

func (db *projectsMockDB) GetProject(_ context.Context, id string) (*models.Project, error) {
	for _, p := range db.projects {
		if p.ID == id {
			return p, nil
		}
	}

	return nil, nil
}

func (db *projectsMockDB) CreateProject(_ context.Context, project *models.Project) error {
	for _, p := range db.projects {
		if p.Name == project.Name || p.Slug == project.Slug {
			return &pgconn.PgError{Code: "23505"}
		}
	}

	project.ID = "new-project"
	db.createdProject = project
	db.projects[project.Name] = project

	return nil
}

func (db *projectsMockDB) UpdateProject(_ context.Context, project *models.Project) error {
	db.updated = project
	return nil
}

func (db *projectsMockDB) DeleteProject(_ context.Context, id string) error {
	if db.deleteErr != nil {
		return db.deleteErr
	}

	db.deleted = append(db.deleted, id)

	return nil
}

func (db *projectsMockDB) ListProjects(_ context.Context, userID string) ([]*models.Project, error) {
	return db.userProjects[userID], nil
}

func (db *projectsMockDB) ListAllProjects(context.Context) ([]*models.Project, error) {
	db.listedAll = true

	out := make([]*models.Project, 0, len(db.projects))
	for _, p := range db.projects {
		out = append(out, p)
	}

	return out, nil
}

func (db *projectsMockDB) AddProjectMember(_ context.Context, projectID, userID, role string) error {
	db.members[userID] = &models.ProjectMember{ID: "m-" + userID, ProjectID: projectID, UserID: userID, Role: role, CreatedAt: time.Now()}
	return nil
}

func (db *projectsMockDB) GetProjectMember(_ context.Context, projectID, userID string) (*models.ProjectMember, error) {
	if m := db.members[userID]; m != nil && m.ProjectID == projectID {
		return m, nil
	}

	return nil, nil
}

func (db *projectsMockDB) ListProjectMembers(_ context.Context, projectID string) ([]*models.ProjectMember, error) {
	var out []*models.ProjectMember

	for _, m := range db.members {
		if m.ProjectID == projectID {
			out = append(out, m)
		}
	}

	return out, nil
}

func (db *projectsMockDB) UpdateProjectMemberRole(_ context.Context, _, userID, role string) error {
	db.roleUpdates[userID] = role
	db.members[userID].Role = role

	return nil
}

func (db *projectsMockDB) RemoveProjectMember(_ context.Context, _, userID string) error {
	db.removed = append(db.removed, userID)
	delete(db.members, userID)

	return nil
}

func (db *projectsMockDB) GetUserByID(_ context.Context, id string) (*models.User, error) {
	return db.users[id], nil
}

func (db *projectsMockDB) GetGroupByID(_ context.Context, id string) (*models.Group, error) {
	return db.groups[id], nil
}

func (db *projectsMockDB) ListProjectGroups(_ context.Context, _ string) ([]*models.ProjectGroup, error) {
	var out []*models.ProjectGroup
	for _, g := range db.projectGroup {
		out = append(out, g)
	}

	return out, nil
}

func (db *projectsMockDB) GetProjectGroup(_ context.Context, _, groupID string) (*models.ProjectGroup, error) {
	return db.projectGroup[groupID], nil
}

func (db *projectsMockDB) SetProjectGroup(_ context.Context, projectID, groupID, role string, createdBy *string) error {
	db.setGroups[groupID] = role
	db.setGroupBy = createdBy
	db.projectGroup[groupID] = &models.ProjectGroup{ProjectID: projectID, GroupID: groupID, GroupName: db.groups[groupID].Name, Role: role, CreatedAt: time.Now()}

	return nil
}

func (db *projectsMockDB) RemoveProjectGroup(_ context.Context, _, groupID string) error {
	db.removedGroups = append(db.removedGroups, groupID)
	delete(db.projectGroup, groupID)

	return nil
}

func (db *projectsMockDB) GetRepositoryByName(_ context.Context, name string) (*models.Repository, error) {
	if db.repo != nil && db.repo.Name == name {
		cp := *db.repo
		return &cp, nil
	}

	return nil, nil
}

func (db *projectsMockDB) UpdateRepository(_ context.Context, repo *models.Repository) error {
	db.updRepo = repo
	return nil
}

// newProjectsMock builds project "acme" owned by owner-1 with members:
// owner-1/owner-2 (owner), maint-1/maint-2 (maintainer), dev-1 (developer),
// view-1 (viewer). admin-1 is a global admin who is not a member.
func newProjectsMock() *projectsMockDB {
	owner := "owner-1"
	display := "Acme"
	desc := "old"

	acme := &models.Project{
		ID: testAcmeID, Name: "acme", Slug: "acme", OwnerID: &owner,
		DisplayName: &display, Description: &desc,
		AllowedFormats:   []string{"maven"},
		AllowedRepoTypes: []string{models.ProjectRepoTypeHosted},
		RepoNamePrefix:   "acme-", MaxRepositories: 10, TokenMaxTTLDays: 30,
	}

	db := &projectsMockDB{
		projects:     map[string]*models.Project{"acme": acme},
		members:      map[string]*models.ProjectMember{},
		users:        map[string]*models.User{},
		groups:       map[string]*models.Group{"g-eng": {ID: "g-eng", Name: "eng"}},
		projectGroup: map[string]*models.ProjectGroup{},
		admins:       map[string]bool{"admin-1": true},
		userProjects: map[string][]*models.Project{},
		roleUpdates:  map[string]string{},
		setGroups:    map[string]string{},
	}

	for id, role := range map[string]string{
		"owner-1": models.ProjectRoleOwner, "owner-2": models.ProjectRoleOwner,
		"maint-1": models.ProjectRoleMaintainer, "maint-2": models.ProjectRoleMaintainer,
		"dev-1": models.ProjectRoleDeveloper, "view-1": models.ProjectRoleViewer,
	} {
		db.members[id] = &models.ProjectMember{ID: "m-" + id, ProjectID: testAcmeID, UserID: id, Role: role}
		db.users[id] = &models.User{ID: id, Username: id}
	}

	db.users["newbie"] = &models.User{ID: "newbie", Username: "newbie"}

	return db
}

func newProjectsRouter(t *testing.T, db *projectsMockDB, withEvaluator bool) http.Handler {
	t.Helper()

	log := newFixTestLogger(t)

	var ev *auth.Evaluator
	if withEvaluator {
		ev = auth.NewEvaluator(db, log, false)
	}

	router := chi.NewRouter()
	NewProjectsHandler(db, log, ev).RegisterRoutes(router)

	return router
}

func projectJSON(t *testing.T, router http.Handler, userID, method, target, body string) (int, map[string]any) {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	req = req.WithContext(middleware.WithPrincipal(req.Context(), &auth.Principal{UserID: &userID, Username: userID}))

	rw := httptest.NewRecorder()
	router.ServeHTTP(rw, req)

	out := map[string]any{}
	_ = json.Unmarshal(rw.Body.Bytes(), &out)

	return rw.Code, out
}

func checkCreateDefaults(t *testing.T, p *models.Project) {
	t.Helper()

	if p.RepoNamePrefix != "my-proj-" || p.MaxRepositories != 50 || p.TokenMaxTTLDays != 365 {
		t.Errorf("defaults = prefix %q max %d ttl %d", p.RepoNamePrefix, p.MaxRepositories, p.TokenMaxTTLDays)
	}

	if !slices.Equal(p.AllowedRepoTypes, []string{models.ProjectRepoTypeHosted}) {
		t.Errorf("allowed_repo_types = %v, want [hosted]", p.AllowedRepoTypes)
	}

	if p.AllowedFormats != nil || p.AllowPublicRepos {
		t.Errorf("allowed_formats = %v public = %v, want nil/false", p.AllowedFormats, p.AllowPublicRepos)
	}

	if p.OwnerID == nil || *p.OwnerID != "admin-1" {
		t.Errorf("owner = %v, want admin-1", p.OwnerID)
	}
}

func checkCreateWithPolicy(t *testing.T, p *models.Project) {
	t.Helper()

	if !slices.Equal(p.AllowedFormats, []string{"go", "raw"}) || !slices.Equal(p.AllowedRepoTypes, []string{"hosted", "proxy"}) {
		t.Errorf("lists = %v / %v", p.AllowedFormats, p.AllowedRepoTypes)
	}

	if p.RepoNamePrefix != "team_" || !p.AllowPublicRepos || p.MaxRepositories != 3 || p.TokenMaxTTLDays != 0 {
		t.Errorf("policy = %+v", p)
	}
}

func checkNilFormats(t *testing.T, p *models.Project) {
	t.Helper()

	if p.AllowedFormats != nil {
		t.Errorf("allowed_formats = %#v, want nil", p.AllowedFormats)
	}
}

func checkEmptyPrefix(t *testing.T, p *models.Project) {
	t.Helper()

	if p.RepoNamePrefix != "" {
		t.Errorf("prefix = %q, want empty", p.RepoNamePrefix)
	}
}

func checkDisplayNameNew(t *testing.T, p *models.Project) {
	t.Helper()

	if p.DisplayName == nil || *p.DisplayName != "New" {
		t.Errorf("display_name = %v", p.DisplayName)
	}
}

func checkUpdateKeepsOmitted(t *testing.T, p *models.Project) {
	t.Helper()

	if *p.Description != "d" || *p.DisplayName != "Acme" {
		t.Errorf("description/display = %q/%q", *p.Description, *p.DisplayName)
	}

	if !slices.Equal(p.AllowedFormats, []string{"maven"}) || p.RepoNamePrefix != "acme-" || p.MaxRepositories != 10 || p.TokenMaxTTLDays != 30 {
		t.Errorf("policy changed: %+v", p)
	}
}

func checkAdminPolicyUpdate(t *testing.T, p *models.Project) {
	t.Helper()

	if !slices.Equal(p.AllowedFormats, []string{"raw", "go"}) || !slices.Equal(p.AllowedRepoTypes, []string{"hosted", "group"}) ||
		p.RepoNamePrefix != "" || !p.AllowPublicRepos || p.MaxRepositories != 0 || p.TokenMaxTTLDays != 7 {
		t.Errorf("policy = %+v", p)
	}
}

func TestProjectsCreatePolicy(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		check      func(t *testing.T, p *models.Project)
	}{
		{"defaults", `{"name":"My_Proj"}`, http.StatusCreated, checkCreateDefaults},
		{
			"with policy", `{"name":"x","allowed_formats":["go","raw","go"],"allowed_repo_types":["hosted","proxy"],"repo_name_prefix":"team_","allow_public_repos":true,"max_repositories":3,"token_max_ttl_days":0}`,
			http.StatusCreated, checkCreateWithPolicy,
		},
		{"empty allowed_formats means every format", `{"name":"x","allowed_formats":[]}`, http.StatusCreated, checkNilFormats},
		{"empty prefix allowed", `{"name":"x","repo_name_prefix":""}`, http.StatusCreated, checkEmptyPrefix},
		{"unknown format", `{"name":"x","allowed_formats":["gomod"]}`, http.StatusBadRequest, nil},
		{"empty repo types", `{"name":"x","allowed_repo_types":[]}`, http.StatusBadRequest, nil},
		{"invalid repo type", `{"name":"x","allowed_repo_types":["virtual"]}`, http.StatusBadRequest, nil},
		{"bad prefix", `{"name":"x","repo_name_prefix":"a b/"}`, http.StatusBadRequest, nil},
		{"negative max", `{"name":"x","max_repositories":-1}`, http.StatusBadRequest, nil},
		{"negative ttl", `{"name":"x","token_max_ttl_days":-5}`, http.StatusBadRequest, nil},
		{"max overflow", `{"name":"x","max_repositories":4294967296}`, http.StatusBadRequest, nil},
		{"wrong type", `{"name":"x","max_repositories":"many"}`, http.StatusBadRequest, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			router := newProjectsRouter(t, db, true)

			code, body := projectJSON(t, router, "admin-1", http.MethodPost, "/api/v1/projects", tt.body)
			if code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.wantStatus, body)
			}

			if tt.check == nil {
				if db.createdProject != nil {
					t.Error("project was created despite the error")
				}

				return
			}

			tt.check(t, db.createdProject)

			if m := db.members["admin-1"]; m == nil || m.Role != models.ProjectRoleOwner {
				t.Errorf("creator membership = %+v, want owner", m)
			}
		})
	}
}

func TestProjectsCreateResponse(t *testing.T) {
	db := newProjectsMock()
	router := newProjectsRouter(t, db, true)

	code, body := projectJSON(t, router, "admin-1", http.MethodPost, "/api/v1/projects", `{"name":"Fresh"}`)
	if code != http.StatusCreated {
		t.Fatalf("status = %d; body = %v", code, body)
	}

	if body["repo_name_prefix"] != "fresh-" || body["max_repositories"] != float64(50) || body["token_max_ttl_days"] != float64(365) {
		t.Errorf("response policy = %v", body)
	}

	if body["allowed_formats"] != nil {
		t.Errorf("allowed_formats = %v, want null", body["allowed_formats"])
	}

	if _, ok := body["member_role"]; ok {
		t.Error("response still carries member_role")
	}
}

func TestProjectsCreateSlug(t *testing.T) {
	long := strings.Repeat("a", 65)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantSlug   string
	}{
		{"slug derived from name", `{"name":"My_Proj"}`, http.StatusCreated, "my-proj"},
		{"duplicate name", `{"name":"acme"}`, http.StatusConflict, ""},
		{"slug-only collision (case)", `{"name":"Acme"}`, http.StatusConflict, ""},
		{"bad name", `{"name":"no spaces"}`, http.StatusBadRequest, ""},
		{"name without letters or digits", `{"name":"---"}`, http.StatusBadRequest, ""},
		{"slug too long", `{"name":"` + long + `"}`, http.StatusBadRequest, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newProjectsRouter(t, newProjectsMock(), true)

			code, body := projectJSON(t, router, "admin-1", http.MethodPost, "/api/v1/projects", tt.body)
			if code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.wantStatus, body)
			}

			if tt.wantSlug != "" && body["slug"] != tt.wantSlug {
				t.Errorf("slug = %v, want %s", body["slug"], tt.wantSlug)
			}
		})
	}
}

func TestProjectsUpdatePolicy(t *testing.T) {
	tests := []struct {
		name          string
		user          string
		withEvaluator bool
		body          string
		wantStatus    int
		check         func(t *testing.T, p *models.Project)
	}{
		{"owner display_name ok", "owner-1", true, `{"display_name":"New"}`, http.StatusOK, checkDisplayNameNew},
		{"maintainer display_name ok", "maint-1", true, `{"display_name":"New"}`, http.StatusOK, nil},
		{"no evaluator display_name ok", "owner-1", false, `{"display_name":"New"}`, http.StatusOK, nil},
		{"update keeps omitted fields", "owner-1", true, `{"description":"d"}`, http.StatusOK, checkUpdateKeepsOmitted},
		{"owner member policy 403 (allowed_formats)", "owner-1", true, `{"allowed_formats":["raw"]}`, http.StatusForbidden, nil},
		{"owner member policy 403 (repo types)", "owner-1", true, `{"allowed_repo_types":["proxy"]}`, http.StatusForbidden, nil},
		{"owner member policy 403 (prefix)", "owner-1", true, `{"repo_name_prefix":"x-"}`, http.StatusForbidden, nil},
		{"owner member policy 403 (public)", "owner-1", true, `{"allow_public_repos":true}`, http.StatusForbidden, nil},
		{"owner member policy 403 (max)", "owner-1", true, `{"max_repositories":999}`, http.StatusForbidden, nil},
		{"owner member policy 403 (ttl)", "owner-1", true, `{"token_max_ttl_days":9999}`, http.StatusForbidden, nil},
		{"policy with display_name still 403", "owner-1", true, `{"display_name":"x","max_repositories":1}`, http.StatusForbidden, nil},
		{"no evaluator policy 403", "owner-1", false, `{"max_repositories":1}`, http.StatusForbidden, nil},
		{"maintainer policy 403", "maint-1", true, `{"max_repositories":1}`, http.StatusForbidden, nil},
		{"developer 403", "dev-1", true, `{"display_name":"x"}`, http.StatusForbidden, nil},
		{"viewer 403", "view-1", true, `{"display_name":"x"}`, http.StatusForbidden, nil},
		{"outsider 403", "newbie", true, `{"display_name":"x"}`, http.StatusForbidden, nil},
		{"admin non-member policy ok", "admin-1", true, `{"allowed_formats":["raw","go"],"allowed_repo_types":["hosted","group"],"repo_name_prefix":"","allow_public_repos":true,"max_repositories":0,"token_max_ttl_days":7}`, http.StatusOK, checkAdminPolicyUpdate},
		{"admin empty allowed_formats stores nil", "admin-1", true, `{"allowed_formats":[]}`, http.StatusOK, checkNilFormats},
		{"admin unknown format 400", "admin-1", true, `{"allowed_formats":["nope"]}`, http.StatusBadRequest, nil},
		{"admin empty repo types 400", "admin-1", true, `{"allowed_repo_types":[]}`, http.StatusBadRequest, nil},
		{"admin bad repo type 400", "admin-1", true, `{"allowed_repo_types":["x"]}`, http.StatusBadRequest, nil},
		{"admin bad prefix 400", "admin-1", true, `{"repo_name_prefix":"a/b"}`, http.StatusBadRequest, nil},
		{"admin negative max 400", "admin-1", true, `{"max_repositories":-1}`, http.StatusBadRequest, nil},
		{"admin negative ttl 400", "admin-1", true, `{"token_max_ttl_days":-1}`, http.StatusBadRequest, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			router := newProjectsRouter(t, db, tt.withEvaluator)

			code, body := projectJSON(t, router, tt.user, http.MethodPut, "/api/v1/projects/acme", tt.body)
			if code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.wantStatus, body)
			}

			if tt.wantStatus != http.StatusOK {
				if db.updated != nil {
					t.Error("project updated despite the error")
				}

				return
			}

			if db.updated == nil {
				t.Fatal("project not updated")
			}

			if tt.check != nil {
				tt.check(t, db.updated)
			}
		})
	}
}

func TestProjectsAdminBypassAndDelete(t *testing.T) {
	tests := []struct {
		name          string
		user          string
		withEvaluator bool
		method        string
		target        string
		body          string
		want          int
	}{
		{"admin non-member GET", "admin-1", true, http.MethodGet, "/api/v1/projects/acme", "", http.StatusOK},
		{"admin non-member PUT", "admin-1", true, http.MethodPut, "/api/v1/projects/acme", `{"display_name":"x"}`, http.StatusOK},
		{"admin non-member DELETE", "admin-1", true, http.MethodDelete, "/api/v1/projects/acme", "", http.StatusNoContent},
		{"admin non-member lists members", "admin-1", true, http.MethodGet, "/api/v1/projects/acme/members", "", http.StatusOK},
		{"admin without evaluator GET", "admin-1", false, http.MethodGet, "/api/v1/projects/acme", "", http.StatusForbidden},
		{"outsider GET", "newbie", true, http.MethodGet, "/api/v1/projects/acme", "", http.StatusForbidden},
		{"outsider DELETE", "newbie", true, http.MethodDelete, "/api/v1/projects/acme", "", http.StatusForbidden},
		{"viewer GET", "view-1", true, http.MethodGet, "/api/v1/projects/acme", "", http.StatusOK},
		{"maintainer DELETE", "maint-1", true, http.MethodDelete, "/api/v1/projects/acme", "", http.StatusForbidden},
		{"non-recorded owner DELETE", "owner-2", true, http.MethodDelete, "/api/v1/projects/acme", "", http.StatusForbidden},
		{"recorded owner DELETE", "owner-1", true, http.MethodDelete, "/api/v1/projects/acme", "", http.StatusNoContent},
		{"unknown project", "admin-1", true, http.MethodGet, "/api/v1/projects/ghost", "", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			router := newProjectsRouter(t, db, tt.withEvaluator)

			code, body := projectJSON(t, router, tt.user, tt.method, tt.target, tt.body)
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if tt.method == http.MethodDelete && (tt.want == http.StatusNoContent) != (len(db.deleted) == 1) {
				t.Errorf("deleted = %v", db.deleted)
			}
		})
	}
}

func TestProjectsDeleteStillOwnsRepositories(t *testing.T) {
	db := newProjectsMock()
	db.deleteErr = &pgconn.PgError{Code: "23503"}
	router := newProjectsRouter(t, db, true)

	code, body := projectJSON(t, router, "owner-1", http.MethodDelete, "/api/v1/projects/acme", "")
	if code != http.StatusConflict || body["code"] != "conflict" {
		t.Fatalf("status/body = %d/%v, want 409/conflict", code, body)
	}

	if msg, _ := body["error"].(string); !strings.Contains(msg, "in the trash") {
		t.Errorf("message %q does not mention repositories in the trash", msg)
	}
}

func TestProjectsList(t *testing.T) {
	tests := []struct {
		name          string
		user          string
		withEvaluator bool
		target        string
		want          int
		wantAll       bool
		wantNames     []string
	}{
		{"member list", "dev-1", true, "/api/v1/projects", http.StatusOK, false, []string{"acme"}},
		{"all=true non-admin", "dev-1", true, "/api/v1/projects?all=true", http.StatusForbidden, false, nil},
		{"all=true owner member", "owner-1", true, "/api/v1/projects?all=true", http.StatusForbidden, false, nil},
		{"all=true without evaluator", "admin-1", false, "/api/v1/projects?all=true", http.StatusForbidden, false, nil},
		{"all=true admin", "admin-1", true, "/api/v1/projects?all=true", http.StatusOK, true, []string{"acme"}},
		{"all=false admin lists own", "admin-1", true, "/api/v1/projects?all=false", http.StatusOK, false, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			db.userProjects["dev-1"] = []*models.Project{db.projects["acme"]}
			router := newProjectsRouter(t, db, tt.withEvaluator)

			code, body := projectJSON(t, router, tt.user, http.MethodGet, tt.target, "")
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if db.listedAll != tt.wantAll {
				t.Errorf("ListAllProjects called = %v, want %v", db.listedAll, tt.wantAll)
			}

			if tt.wantNames == nil {
				return
			}

			list, _ := body["projects"].([]any)
			if len(list) != len(tt.wantNames) {
				t.Fatalf("projects = %v, want %v", list, tt.wantNames)
			}
		})
	}
}

func TestProjectsAddMember(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		body     string
		want     int
		wantRole string
	}{
		{"owner grants owner", "owner-1", `{"user_id":"newbie","role":"owner"}`, http.StatusCreated, "owner"},
		{"owner grants maintainer", "owner-1", `{"user_id":"newbie","role":"maintainer"}`, http.StatusCreated, "maintainer"},
		{"maintainer grants maintainer", "maint-1", `{"user_id":"newbie","role":"maintainer"}`, http.StatusForbidden, ""},
		{"maintainer grants owner", "maint-1", `{"user_id":"newbie","role":"owner"}`, http.StatusForbidden, ""},
		{"maintainer grants developer", "maint-1", `{"user_id":"newbie","role":"developer"}`, http.StatusCreated, "developer"},
		{"maintainer grants viewer", "maint-1", `{"user_id":"newbie","role":"viewer"}`, http.StatusCreated, "viewer"},
		{"role defaults to viewer", "maint-1", `{"user_id":"newbie"}`, http.StatusCreated, "viewer"},
		{"developer cannot add", "dev-1", `{"user_id":"newbie","role":"viewer"}`, http.StatusForbidden, ""},
		{"viewer cannot add", "view-1", `{"user_id":"newbie","role":"viewer"}`, http.StatusForbidden, ""},
		{"global admin grants owner", "admin-1", `{"user_id":"newbie","role":"owner"}`, http.StatusCreated, "owner"},
		{"old admin role rejected", "owner-1", `{"user_id":"newbie","role":"admin"}`, http.StatusBadRequest, ""},
		{"old member role rejected", "owner-1", `{"user_id":"newbie","role":"member"}`, http.StatusBadRequest, ""},
		{"unknown user", "owner-1", `{"user_id":"ghost","role":"viewer"}`, http.StatusNotFound, ""},
		{"already a member", "owner-1", `{"user_id":"dev-1","role":"viewer"}`, http.StatusConflict, ""},
		{"invalid body", "owner-1", `nope`, http.StatusBadRequest, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			router := newProjectsRouter(t, db, true)

			code, body := projectJSON(t, router, tt.user, http.MethodPost, "/api/v1/projects/acme/members", tt.body)
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if tt.wantRole == "" {
				if m := db.members["newbie"]; m != nil {
					t.Errorf("member added despite error: %+v", m)
				}

				return
			}

			if body["role"] != tt.wantRole || db.members["newbie"].Role != tt.wantRole {
				t.Errorf("role = %v / %v, want %s", body["role"], db.members["newbie"], tt.wantRole)
			}
		})
	}
}

// dropMember removes a member row, leaving the recorded owner and everybody
// else untouched.
func dropMember(db *projectsMockDB, id string) { delete(db.members, id) }

func TestProjectsUpdateMemberRole(t *testing.T) {
	tests := []struct {
		name   string
		user   string
		target string
		body   string
		setup  func(db *projectsMockDB)
		want   int
	}{
		{"owner promotes developer to maintainer", "owner-1", "dev-1", `{"role":"maintainer"}`, nil, http.StatusOK},
		{"owner grants owner", "owner-1", "dev-1", `{"role":"owner"}`, nil, http.StatusOK},
		{"owner demotes maintainer", "owner-1", "maint-1", `{"role":"viewer"}`, nil, http.StatusOK},
		{"owner demotes another owner", "owner-1", "owner-2", `{"role":"developer"}`, nil, http.StatusOK},
		{"maintainer grants maintainer", "maint-1", "dev-1", `{"role":"maintainer"}`, nil, http.StatusForbidden},
		{"maintainer grants owner", "maint-1", "dev-1", `{"role":"owner"}`, nil, http.StatusForbidden},
		{"maintainer lowers developer to viewer", "maint-1", "dev-1", `{"role":"viewer"}`, nil, http.StatusOK},
		{"maintainer raises viewer to developer", "maint-1", "view-1", `{"role":"developer"}`, nil, http.StatusOK},
		{"maintainer cannot demote maintainer", "maint-1", "maint-2", `{"role":"developer"}`, nil, http.StatusForbidden},
		{"maintainer cannot demote owner", "maint-1", "owner-2", `{"role":"developer"}`, nil, http.StatusForbidden},
		{"maintainer same role is a no-op", "maint-1", "maint-2", `{"role":"maintainer"}`, nil, http.StatusOK},
		{"developer cannot change", "dev-1", "view-1", `{"role":"developer"}`, nil, http.StatusForbidden},
		{"recorded owner is immutable", "owner-2", "owner-1", `{"role":"maintainer"}`, nil, http.StatusConflict},
		{"global admin cannot demote recorded owner", "admin-1", "owner-1", `{"role":"viewer"}`, nil, http.StatusConflict},
		{"last owner cannot be demoted", "owner-1", "owner-2", `{"role":"developer"}`, func(db *projectsMockDB) { dropMember(db, "owner-1") }, http.StatusConflict},
		{"invalid role admin", "owner-1", "dev-1", `{"role":"admin"}`, nil, http.StatusBadRequest},
		{"invalid role member", "owner-1", "dev-1", `{"role":"member"}`, nil, http.StatusBadRequest},
		{"empty role", "owner-1", "dev-1", `{}`, nil, http.StatusBadRequest},
		{"unknown member", "owner-1", "ghost", `{"role":"viewer"}`, nil, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			if tt.setup != nil {
				tt.setup(db)
			}

			router := newProjectsRouter(t, db, true)

			code, body := projectJSON(t, router, tt.user, http.MethodPut, "/api/v1/projects/acme/members/"+tt.target, tt.body)
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if tt.want >= http.StatusBadRequest && len(db.roleUpdates) != 0 {
				t.Errorf("role updated despite the error: %v", db.roleUpdates)
			}
		})
	}
}

func TestProjectsRemoveMemberProtectsOwners(t *testing.T) {
	tests := []struct {
		name   string
		user   string
		target string
		setup  func(db *projectsMockDB)
		want   int
	}{
		{"owner removes maintainer", "owner-1", "maint-1", nil, http.StatusNoContent},
		{"owner removes developer", "owner-1", "dev-1", nil, http.StatusNoContent},
		{"owner removes another owner", "owner-1", "owner-2", nil, http.StatusNoContent},
		{"owner cannot remove the recorded owner", "owner-2", "owner-1", nil, http.StatusConflict},
		{"recorded owner cannot remove themselves", "owner-1", "owner-1", nil, http.StatusConflict},
		{"global admin cannot remove recorded owner", "admin-1", "owner-1", nil, http.StatusConflict},
		{"global admin removes a second owner", "admin-1", "owner-2", nil, http.StatusNoContent},
		{"last owner cannot be removed", "owner-1", "owner-2", func(db *projectsMockDB) { dropMember(db, "owner-1") }, http.StatusConflict},
		{"maintainer removes developer", "maint-1", "dev-1", nil, http.StatusNoContent},
		{"maintainer removes viewer", "maint-1", "view-1", nil, http.StatusNoContent},
		{"maintainer cannot remove maintainer", "maint-1", "maint-2", nil, http.StatusForbidden},
		{"maintainer cannot remove owner", "maint-1", "owner-2", nil, http.StatusForbidden},
		{"developer cannot remove anyone", "dev-1", "view-1", nil, http.StatusForbidden},
		{"developer cannot remove maintainer", "dev-1", "maint-1", nil, http.StatusForbidden},
		{"unknown member", "owner-1", "ghost", nil, http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			if tt.setup != nil {
				tt.setup(db)
			}

			router := newProjectsRouter(t, db, true)

			code, body := projectJSON(t, router, tt.user, http.MethodDelete, "/api/v1/projects/acme/members/"+tt.target, "")
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if (tt.want == http.StatusNoContent) != slices.Equal(db.removed, []string{tt.target}) {
				t.Errorf("removed = %v", db.removed)
			}
		})
	}
}

func TestProjectsGroupsList(t *testing.T) {
	tests := []struct {
		name string
		user string
		want int
	}{
		{"viewer lists", "view-1", http.StatusOK},
		{"admin non-member lists", "admin-1", http.StatusOK},
		{"outsider denied", "newbie", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			db.projectGroup["g-eng"] = &models.ProjectGroup{ProjectID: testAcmeID, GroupID: "g-eng", GroupName: "eng", Role: models.ProjectRoleDeveloper}
			router := newProjectsRouter(t, db, true)

			code, body := projectJSON(t, router, tt.user, http.MethodGet, "/api/v1/projects/acme/groups", "")
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if tt.want != http.StatusOK {
				return
			}

			list, _ := body["groups"].([]any)
			if len(list) != 1 {
				t.Fatalf("groups = %v", body["groups"])
			}

			g, _ := list[0].(map[string]any)
			if g["group_name"] != "eng" || g["role"] != "developer" {
				t.Errorf("group = %v", g)
			}
		})
	}
}

func TestProjectsSetGroup(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		group    string
		body     string
		existing *models.ProjectGroup
		want     int
		wantRole string
	}{
		{"maintainer attaches viewer", "maint-1", "g-eng", `{"role":"viewer"}`, nil, http.StatusOK, "viewer"},
		{"maintainer attaches developer", "maint-1", "g-eng", `{"role":"developer"}`, nil, http.StatusOK, "developer"},
		{"maintainer attaches maintainer", "maint-1", "g-eng", `{"role":"maintainer"}`, nil, http.StatusOK, "maintainer"},
		{"owner attaches maintainer", "owner-1", "g-eng", `{"role":"maintainer"}`, nil, http.StatusOK, "maintainer"},
		{"global admin attaches viewer", "admin-1", "g-eng", `{"role":"viewer"}`, nil, http.StatusOK, "viewer"},
		{"groups never owner", "owner-1", "g-eng", `{"role":"owner"}`, nil, http.StatusBadRequest, ""},
		{"old admin role", "owner-1", "g-eng", `{"role":"admin"}`, nil, http.StatusBadRequest, ""},
		{"empty role", "owner-1", "g-eng", `{}`, nil, http.StatusBadRequest, ""},
		{"unknown group", "owner-1", "g-ghost", `{"role":"viewer"}`, nil, http.StatusNotFound, ""},
		{"developer cannot attach", "dev-1", "g-eng", `{"role":"viewer"}`, nil, http.StatusForbidden, ""},
		{"outsider cannot attach", "newbie", "g-eng", `{"role":"viewer"}`, nil, http.StatusForbidden, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			if tt.existing != nil {
				cp := *tt.existing
				db.projectGroup[tt.existing.GroupID] = &cp
			}

			router := newProjectsRouter(t, db, true)

			code, body := projectJSON(t, router, tt.user, http.MethodPut, "/api/v1/projects/acme/groups/"+tt.group, tt.body)
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if tt.wantRole == "" {
				if len(db.setGroups) != 0 {
					t.Errorf("group set despite the error: %v", db.setGroups)
				}

				return
			}

			if body["role"] != tt.wantRole || db.setGroups[tt.group] != tt.wantRole {
				t.Errorf("role = %v / %v, want %s", body["role"], db.setGroups, tt.wantRole)
			}

			if db.setGroupBy == nil || *db.setGroupBy != tt.user {
				t.Errorf("created_by = %v, want %s", db.setGroupBy, tt.user)
			}
		})
	}
}

func TestProjectsRemoveGroup(t *testing.T) {
	tests := []struct {
		name string
		user string
		role string
		want int
	}{
		{"maintainer removes developer group", "maint-1", models.ProjectRoleDeveloper, http.StatusNoContent},
		{"owner removes maintainer group", "owner-1", models.ProjectRoleMaintainer, http.StatusNoContent},
		{"developer cannot remove", "dev-1", models.ProjectRoleViewer, http.StatusForbidden},
		{"global admin removes", "admin-1", models.ProjectRoleMaintainer, http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			db.projectGroup["g-eng"] = &models.ProjectGroup{ProjectID: testAcmeID, GroupID: "g-eng", Role: tt.role}
			router := newProjectsRouter(t, db, true)

			code, body := projectJSON(t, router, tt.user, http.MethodDelete, "/api/v1/projects/acme/groups/g-eng", "")
			if code != tt.want {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.want, body)
			}

			if (tt.want == http.StatusNoContent) != (len(db.removedGroups) == 1) {
				t.Errorf("removed groups = %v", db.removedGroups)
			}
		})
	}

	t.Run("group not attached", func(t *testing.T) {
		router := newProjectsRouter(t, newProjectsMock(), true)

		if code, body := projectJSON(t, router, "owner-1", http.MethodDelete, "/api/v1/projects/acme/groups/g-eng", ""); code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body = %v", code, body)
		}
	})
}

func TestRepositoriesProjectAssignment(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		target        string
		body          string
		wantStatus    int
		wantProjectID string // project_id stored; "" when nothing is written
		wantProject   string // project in the response
	}{
		{"create defaults to system project", http.MethodPost, "/api/v1/repositories", `{"name":"r","format":"raw"}`, http.StatusCreated, models.SystemProjectID, ""},
		{"create in named project", http.MethodPost, "/api/v1/repositories", `{"name":"r","format":"raw","project":"acme"}`, http.StatusCreated, testAcmeID, "acme"},
		{"create in unknown project", http.MethodPost, "/api/v1/repositories", `{"name":"r","format":"raw","project":"ghost"}`, http.StatusNotFound, "", ""},
		{"move to named project", http.MethodPatch, "/api/v1/repository/old", `{"project":"acme"}`, http.StatusOK, testAcmeID, "acme"},
		{"move to unknown project", http.MethodPatch, "/api/v1/repository/old", `{"project":"ghost"}`, http.StatusNotFound, "", ""},
		{"patch without project keeps it", http.MethodPatch, "/api/v1/repository/old", `{"description":"d"}`, http.StatusOK, models.SystemProjectID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newProjectsMock()
			sys := models.SystemProjectID
			db.repo = &models.Repository{ID: "repo-old", Name: "old", Format: "raw", RepoType: "hosted", ProjectID: &sys}

			log := newFixTestLogger(t)
			h := NewRepositoriesHandler(db, log, auth.NewEvaluator(db, log, false))
			router := chi.NewRouter()
			h.RegisterAuthed(router)

			code, body := projectJSON(t, router, "admin-1", tt.method, tt.target, tt.body)
			if code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %v", code, tt.wantStatus, body)
			}

			if tt.wantProjectID == "" {
				return
			}

			stored := db.created
			if tt.method == http.MethodPatch {
				stored = db.updRepo
			}

			if stored == nil || stored.ProjectID == nil || *stored.ProjectID != tt.wantProjectID {
				t.Fatalf("stored project_id = %v, want %s", stored, tt.wantProjectID)
			}

			if got, _ := body["project"].(string); got != tt.wantProject {
				t.Errorf("response project = %q, want %q", got, tt.wantProject)
			}

			if _, ok := body["org"]; ok {
				t.Error("response still carries org")
			}
		})
	}
}

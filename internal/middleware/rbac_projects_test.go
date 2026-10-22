// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

// TestRBACProjectsRoutesAreGlobalManage drives every /api/v1/projects route
// through RBACMiddleware: only a session or access token of a global admin
// passes; deploy tokens and non-admin sessions are denied.
func TestRBACProjectsRoutesAreGlobalManage(t *testing.T) {
	log, err := logger.New("error", "json", "/dev/null")
	if err != nil {
		t.Fatal(err)
	}

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/projects"},
		{http.MethodGet, "/api/v1/projects?all=true"},
		{http.MethodPost, "/api/v1/projects"},
		{http.MethodGet, "/api/v1/projects/acme"},
		{http.MethodPut, "/api/v1/projects/acme"},
		{http.MethodDelete, "/api/v1/projects/acme"},
		{http.MethodGet, "/api/v1/projects/acme/members"},
		{http.MethodPost, "/api/v1/projects/acme/members"},
		{http.MethodPut, "/api/v1/projects/acme/members/u1"},
		{http.MethodDelete, "/api/v1/projects/acme/members/u1"},
		{http.MethodGet, "/api/v1/projects/acme/groups"},
		{http.MethodPut, "/api/v1/projects/acme/groups/g1"},
		{http.MethodDelete, "/api/v1/projects/acme/groups/g1"},
	}

	readOnly := []*models.PermissionTarget{{ID: "t-ro", RepositoryFilter: ".*", PathPattern: ".*", Actions: []string{"read", "write", "delete"}}}
	admin := []*models.PermissionTarget{{ID: "t-admin", RepositoryFilter: ".*", PathPattern: ".*", Actions: []string{"manage", "read", "write", "delete"}}}

	credentials := []struct {
		name      string
		kind      string // token kind; "" = session
		targets   []*models.PermissionTarget
		wantAllow bool
	}{
		{"admin session", "", admin, true},
		{"admin access token", auth.TokenKindAccess, admin, true},
		{"admin deploy token", auth.TokenKindDeploy, admin, false},
		{"non-admin session", "", readOnly, false},
		{"non-admin access token", auth.TokenKindAccess, readOnly, false},
		{"non-admin deploy token", auth.TokenKindDeploy, readOnly, false},
	}

	for _, cred := range credentials {
		for _, route := range routes {
			t.Run(cred.name+" "+route.method+" "+route.path, func(t *testing.T) {
				db := &benchRBACDB{
					targets:  cred.targets,
					repos:    map[string]*models.Repository{},
					tokens:   map[string]*models.Token{},
					users:    map[string]*models.User{"user-1": {ID: "user-1", Username: "jane"}},
					sessions: map[string]*models.Session{},
				}

				req := httptest.NewRequestWithContext(t.Context(), route.method, route.path, http.NoBody)

				if cred.kind == "" {
					value, sess, err := auth.CreateSession(t.Context(), db, "user-1", "", "")
					if err != nil {
						t.Fatal(err)
					}

					db.sessions[sess.TokenHash] = sess
					req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: value})
				} else {
					raw, tok, err := auth.CreateToken(t.Context(), db, cred.kind, "ci", "user-1", nil, nil)
					if err != nil {
						t.Fatal(err)
					}

					now := time.Now()
					tok.LastUsedAt = &now
					db.tokens[tok.TokenHash] = tok
					req.Header.Set("Authorization", "Bearer "+raw)
				}

				next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
				handler := RBACMiddleware(RBACConfig{}, db, auth.NewEvaluator(db, log, false), log)(next)

				rw := httptest.NewRecorder()
				handler.ServeHTTP(rw, req)

				if allowed := rw.Code == http.StatusOK; allowed != cred.wantAllow {
					t.Fatalf("status = %d, allowed = %v, want %v", rw.Code, allowed, cred.wantAllow)
				}
			})
		}
	}
}

func TestIsSelfServiceRequestRejectsProjectRoutes(t *testing.T) {
	uid := "user-1"
	session := &auth.Principal{UserID: &uid, Username: "u", Credential: auth.CredentialSession}

	for _, path := range []string{
		"/api/v1/projects", "/api/v1/projects/acme", "/api/v1/projects/acme/members",
		"/api/v1/projects/acme/members/user-1", "/api/v1/projects/acme/groups/g1",
	} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
			if isSelfServiceRequest(req, session) {
				t.Errorf("isSelfServiceRequest(%s %s) = true, want false", method, path)
			}
		}
	}
}

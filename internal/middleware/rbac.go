// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/openartifact/openartifact/internal/audit"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/pkg/logger"
)

type principalCtxKey struct{}

func WithPrincipal(ctx context.Context, p *auth.Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

func PrincipalFromContext(ctx context.Context) *auth.Principal {
	p, _ := ctx.Value(principalCtxKey{}).(*auth.Principal)
	return p
}

type RBACConfig struct {
	LegacyAPIKey        string
	LegacyAPIKeyEnabled bool
	AllowAnonymousRead  bool
	// MCPPath is the mount path for the MCP HTTP endpoint (e.g. "/mcp").
	// It lives outside /api/, /v2/, and /repos/ but must never fall through
	// the SPA/static bypass below — MCP tools require auth like the REST API.
	MCPPath string
}

func RBACMiddleware(cfg RBACConfig, db database.Database, evaluator *auth.Evaluator, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			principal, credential := resolvePrincipal(ctx, r, cfg, db, log)

			// SPA/static paths bypass RBAC. Only the exact known Vue-router
			// page shapes under /repos/ are client-side routing; every other
			// /repos/… path (and /v2/, /api/…, the MCP mount) is a
			// format-handler or REST API endpoint and always requires the
			// full evaluation below.
			if isStaticPath(r.URL.Path, cfg) {
				next.ServeHTTP(w, r.WithContext(WithPrincipal(ctx, principal)))
				return
			}

			if passwordChangeBlocked(r, principal) {
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error":      "password_change_required",
					"change_url": "/api/v1/auth/change-password",
				})

				return
			}

			// Self-service auth endpoints are scoped to the caller's own session
			// and don't require any explicit permission target. Skipping the
			// RBAC evaluator here lets an authenticated user always view their
			// profile, log out, or rotate their own password regardless of the
			// permission_assignments configured for them.
			if !principal.IsAnonymous && isSelfServiceAuthPath(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r.WithContext(WithPrincipal(ctx, principal)))
				return
			}

			action := methodToAction(r.Method, r.URL.Path)
			repoName, artifactPath := extractRepoAndPath(r.URL.Path)

			allowed, err := evaluator.Evaluate(ctx, *principal, repoName, artifactPath, action)
			if err != nil {
				log.Errorw("auth evaluation error",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
				)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "auth_evaluation_failed"})

				return
			}

			if !allowed {
				writeAccessDenied(w, log, principal, credential, repoName, artifactPath, action)

				return
			}

			next.ServeHTTP(w, r.WithContext(WithPrincipal(ctx, principal)))
		})
	}
}

// isStaticPath reports whether urlPath bypasses RBAC: SPA/static paths that
// are neither API, OCI, MCP, nor a format-handler endpoint under /repos/.
func isStaticPath(urlPath string, cfg RBACConfig) bool {
	isRepoSPA := strings.HasPrefix(urlPath, "/repos/") &&
		isSPARepoPath(strings.Split(strings.Trim(urlPath, "/"), "/"))
	isMCPPath := cfg.MCPPath != "" &&
		(urlPath == cfg.MCPPath || strings.HasPrefix(urlPath, cfg.MCPPath+"/"))

	return !strings.HasPrefix(urlPath, "/api/") && !strings.HasPrefix(urlPath, "/v2/") && !isMCPPath &&
		(!strings.HasPrefix(urlPath, "/repos/") || isRepoSPA)
}

// passwordChangeBlocked reports whether the principal must change its password
// first and the request is not one of the two endpoints allowed in that state.
func passwordChangeBlocked(r *http.Request, principal *auth.Principal) bool {
	if !principal.MustChangePassword || principal.IsAnonymous || principal.IsLegacy {
		return false
	}

	return r.Method != http.MethodPost ||
		(r.URL.Path != "/api/v1/auth/change-password" && r.URL.Path != "/api/v1/auth/logout")
}

func writeAccessDenied(w http.ResponseWriter, log *logger.Logger, principal *auth.Principal, credential, repoName, artifactPath, action string) {
	principalName := "anonymous"
	if principal.Username != "" {
		principalName = principal.Username
	}

	log.Warnw("access denied",
		"event", "auth_deny",
		"principal", principalName,
		"credential", credential,
		"repo", repoName,
		"path", artifactPath,
		"action", action,
	)

	if principal.IsAnonymous {
		w.Header().Set("WWW-Authenticate", `Bearer realm="openartifact"`)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})

		return
	}

	writeJSON(w, http.StatusForbidden, map[string]any{
		"error":  "forbidden",
		"action": action,
		"path":   artifactPath,
	})
}

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func resolvePrincipal(ctx context.Context, r *http.Request, cfg RBACConfig, db database.Database, log *logger.Logger) (*auth.Principal, string) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil && cookie.Value != "" {
		session, err := auth.ValidateSession(ctx, db, cookie.Value)
		if err == nil && session != nil {
			user, err := db.GetUserByID(ctx, session.UserID)
			if err == nil && user != nil && !user.Disabled {
				groups, _ := db.ListGroupsByUser(ctx, user.ID)

				groupIDs := make([]string, 0, len(groups))
				for _, g := range groups {
					groupIDs = append(groupIDs, g.ID)
				}

				p := &auth.Principal{
					UserID:             &user.ID,
					Username:           user.Username,
					GroupIDs:           groupIDs,
					IsAnonymous:        false,
					IsLegacy:           false,
					MustChangePassword: user.MustChangePassword,
				}

				//nolint:gosec // G118: audit logging runs in background goroutine by design
				go func() {
					bgCtx := context.Background()
					if extErr := auth.ExtendSession(bgCtx, db, session.ID); extErr != nil {
						log.Debugw("extend session error", "error", extErr, "session_id", session.ID)
					}
				}()

				return p, "session"
			}
		}
	}

	authHeader := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		tokenValue := after
		if strings.HasPrefix(tokenValue, auth.TokenPrefixAccess) || strings.HasPrefix(tokenValue, auth.TokenPrefixDeploy) {
			token, user, err := auth.ValidateToken(ctx, db, tokenValue)

			if al := audit.Default(); al != nil {
				actor := "unknown"
				details := "failure"

				if err == nil && token != nil && user != nil {
					actor = user.Username
					details = "success"
				}

				al.Log(ctx, audit.AuditEvent{
					Action:  "auth.token_validate",
					Actor:   actor,
					Details: details,
				})
			}

			if err == nil && token != nil && user != nil {
				groups, _ := db.ListGroupsByUser(ctx, user.ID)

				groupIDs := make([]string, 0, len(groups))
				for _, g := range groups {
					groupIDs = append(groupIDs, g.ID)
				}

				p := &auth.Principal{
					UserID:             &user.ID,
					Username:           user.Username,
					GroupIDs:           groupIDs,
					IsAnonymous:        false,
					IsLegacy:           false,
					MustChangePassword: user.MustChangePassword,
				}
				auth.UpdateLastUsed(ctx, db, token.ID)

				return p, "bearer_token"
			}
		}
	}

	if cfg.LegacyAPIKeyEnabled && cfg.LegacyAPIKey != "" {
		candidate := extractLegacyKey(r)
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(cfg.LegacyAPIKey)) == 1 {
			log.Warnw("request authenticated via deprecated API_KEY; migrate to user tokens",
				"event", "legacy_api_key_used",
				"path", r.URL.Path,
				"method", r.Method,
				"remote_addr", r.RemoteAddr,
			)

			p := &auth.Principal{
				Username:    "system/legacy-api-key",
				IsAnonymous: false,
				IsLegacy:    true,
			}

			return p, "legacy"
		}
	}

	return &auth.Principal{IsAnonymous: true}, "none"
}

func extractLegacyKey(r *http.Request) string {
	if v := r.Header.Get("X-Api-Key"); v != "" {
		return v
	}

	authHeader := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		return after
	}

	return ""
}

// isSelfServiceAuthPath reports whether the given request targets one of the
// auth endpoints that operate solely on the caller's own session/account and
// should therefore bypass the permission evaluator.
func isSelfServiceAuthPath(method, path string) bool {
	switch path {
	case "/api/v1/auth/me":
		return method == http.MethodGet
	case "/api/v1/auth/change-password":
		return method == http.MethodPost
	case "/api/v1/auth/logout":
		return method == http.MethodPost
	}

	return false
}

func methodToAction(method, path string) string {
	if strings.HasPrefix(path, "/api/v1/") {
		return "manage"
	}

	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return "read"
	case http.MethodDelete:
		return "delete"
	default:
		return "write"
	}
}

// isolatedRepoNamespaces are literal path markers registered as their own
// /repos/<marker>/{name}/… subtree (main.go isolates them to avoid shadowing
// the generic /repos/{name}/… format-handler block). A repository can never
// present a Vue-router page under these — they're always API traffic.
func isolatedRepoNamespaces() []string {
	return []string{"raw", "maven", "sbt", "ivy"}
}

// dockerPathMarkers are the literal sub-resource markers the OCI distribution
// spec defines after the repository name in /v2/<name>/<marker>/….
func dockerPathMarkers() []string {
	return []string{"/manifests/", "/blobs/", "/tags/list", "/referrers/"}
}

// isSPARepoPath reports whether segments (from a "/repos/…" path, split on
// "/") is one of the exact Vue-router page shapes rather than a
// format-handler API call. Everything not explicitly listed here requires
// full RBAC evaluation — deny-by-default, no length-based guessing.
func isSPARepoPath(segments []string) bool {
	if len(segments) < 2 || slices.Contains(isolatedRepoNamespaces(), segments[1]) {
		return false
	}

	switch len(segments) {
	case 2: // /repos/{name}, /repos/new
		return true
	case 3: // /repos/{name}/edit|browse|graph
		switch segments[2] {
		case "edit", "browse", "graph":
			return true
		}

		return false
	case 5: // /repos/{name}/packages/{format}/{pkg}
		return segments[2] == "packages"
	default:
		return false
	}
}

// splitDockerPath splits the portion of a /v2/… URL after the "/v2/" prefix
// into the repository name and the OCI sub-resource path. Docker/OCI
// repository names may themselves contain "/" (namespaced images) and may
// even contain a literal marker segment (e.g. "manifests"), so the split
// point must match what chi's greedy `/v2/{name:*}/manifests/{reference}`
// wildcard actually binds: the LAST occurrence of a known sub-resource
// marker, not the first. Using the first occurrence would truncate such a
// repository's name and desync RBAC's resolved repo from the one the
// handler serves.
func splitDockerPath(rest string) (string, string) {
	if rest == "" {
		return "", ""
	}

	bestIdx := -1

	for _, marker := range dockerPathMarkers() {
		if idx := strings.LastIndex(rest, marker); idx > bestIdx {
			bestIdx = idx
		}
	}

	if bestIdx == -1 {
		return rest, ""
	}

	return rest[:bestIdx], rest[bestIdx+1:]
}

// extractRepoAndPath maps a request path to the (repository, artifact path)
// pair the permission evaluator scopes against. It understands the three
// URL shapes the server actually registers: OCI /v2/… routes, the isolated
// /repos/<marker>/{name}/… namespaces, and the generic /repos/{name}/….
func extractRepoAndPath(urlPath string) (string, string) {
	if rest, ok := strings.CutPrefix(urlPath, "/v2/"); ok {
		return splitDockerPath(rest)
	}

	const repoPrefix = "/repos/"

	after, ok := strings.CutPrefix(urlPath, repoPrefix)
	if !ok {
		return "", urlPath
	}

	for _, marker := range isolatedRepoNamespaces() {
		rest, ok := strings.CutPrefix(after, marker+"/")
		if !ok {
			continue
		}

		before, sub, ok := strings.Cut(rest, "/")
		if !ok {
			return before, ""
		}

		return before, sub
	}

	before, sub, ok := strings.Cut(after, "/")
	if !ok {
		return after, ""
	}

	return before, sub
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if encErr := json.NewEncoder(w).Encode(body); encErr != nil {
		_ = encErr // response header already written
	}
}

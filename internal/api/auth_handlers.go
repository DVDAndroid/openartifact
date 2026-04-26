// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/internal/api/responses"
	"github.com/openartifact/openartifact/internal/auth"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/middleware"
	"github.com/openartifact/openartifact/pkg/logger"
)

func setCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // CSRF double-submit cookie must be readable by JS, so HttpOnly is deliberately unset
		Name:     middleware.CSRFTokenCookieName,
		Value:    token,
		Path:     "/",
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.SessionDuration.Seconds()),
	})
}

func clearCSRFCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // CSRF double-submit cookie must be readable by JS, so HttpOnly is deliberately unset
		Name:     middleware.CSRFTokenCookieName,
		Value:    "",
		Path:     "/",
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}

	s := t.Format(time.RFC3339Nano)

	return &s
}

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
}

func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	entry, ok := rl.entries[ip]
	if !ok || now.Sub(entry.windowStart) > time.Minute {
		rl.entries[ip] = &rateLimitEntry{count: 1, windowStart: now}
		return true
	}

	if entry.count >= 5 {
		return false
	}

	entry.count++

	return true
}

type AuthHandler struct {
	db               database.Database
	log              *logger.Logger
	evaluator        *auth.Evaluator
	loginRateLimiter *ipRateLimiter
	trustedProxies   *middleware.TrustedProxies
}

func NewAuthHandler(db database.Database, log *logger.Logger, evaluator *auth.Evaluator, trustedProxies *middleware.TrustedProxies) *AuthHandler {
	return &AuthHandler{
		db:               db,
		log:              log,
		evaluator:        evaluator,
		loginRateLimiter: &ipRateLimiter{entries: make(map[string]*rateLimitEntry)},
		trustedProxies:   trustedProxies,
	}
}

func (h *AuthHandler) RegisterPublic(r chi.Router) {
	r.Post("/api/v1/auth/login", h.handleLogin)
	r.Post("/api/v1/auth/logout", h.handleLogout)
}

func (h *AuthHandler) RegisterAuthed(r chi.Router) {
	r.Get("/api/v1/auth/me", h.handleMe)
	r.Post("/api/v1/auth/change-password", h.handleChangePassword)
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Real client IP, honoring X-Forwarded-For/X-Real-IP only from a
	// configured trusted proxy — never trust it from the client directly.
	ip := middleware.ClientIP(r, h.trustedProxies)

	// Check rate limit
	if !h.loginRateLimiter.allow(ip) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts")

		return
	}

	var username, password string

	contentType := r.Header.Get("Content-Type")
	if len(contentType) >= 33 && contentType[:33] == "application/x-www-form-urlencoded" {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "failed to parse form")
			return
		}

		username = r.FormValue("username")
		password = r.FormValue("password")
	} else {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}

		username = body.Username
		password = body.Password
	}

	ctx := r.Context()

	user, err := h.db.GetUserByUsername(ctx, username)
	if err != nil {
		h.log.Errorw("login: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if user == nil || user.Disabled || user.PasswordHash == nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}

	ok, err := auth.VerifyPassword(password, *user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}

	userAgent := r.UserAgent()

	csrfToken, err := middleware.NewCSRFToken()
	if err != nil {
		h.log.Errorw("login: generate CSRF token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "session creation failed")
		return
	}

	cookieValue, _, err := auth.CreateSession(ctx, h.db, user.ID, ip, userAgent)
	if err != nil {
		h.log.Errorw("login: create session", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "session creation failed")

		return
	}

	http.SetCookie(w, &http.Cookie{ //nolint:gosec // CSRF double-submit cookie must be readable by JS, so HttpOnly is deliberately unset //nolint:gosec // G124: session cookies are short-lived, Secure flag set by reverse proxy
		Name:     auth.SessionCookieName,
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.SessionDuration.Seconds()),
	})
	setCSRFCookie(w, r, csrfToken)

	now := time.Now()
	user.LastLoginAt = &now
	_ = h.db.UpdateUser(ctx, user)

	writeJSON(w, http.StatusOK, userToResponse(user))
}

func (h *AuthHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = auth.DeleteSession(r.Context(), h.db, cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{ //nolint:gosec // CSRF double-submit cookie must be readable by JS, so HttpOnly is deliberately unset //nolint:gosec // G124: session cookies are short-lived, Secure flag set by reverse proxy
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	clearCSRFCookie(w, r)
	writeNoContent(w)
}

func (h *AuthHandler) handleMe(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	ctx := r.Context()

	user, err := h.db.GetUserByID(ctx, *p.UserID)
	if err != nil {
		h.log.Errorw("me: db error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")

		return
	}

	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "user not found")
		return
	}

	isAdmin, _ := principalIsAdmin(ctx, p, h.evaluator)

	// Include group names for the frontend session store.
	var groupNames []string

	groups, err := h.db.ListGroupsByUser(ctx, user.ID)
	if err == nil {
		groupNames = make([]string, 0, len(groups))
		for _, g := range groups {
			groupNames = append(groupNames, g.Name)
		}
	}

	resp := responses.UserMeResponse{
		ID:                 user.ID,
		Username:           user.Username,
		Email:              user.Email,
		FullName:           user.FullName,
		Disabled:           user.Disabled,
		MustChangePassword: user.MustChangePassword,
		IsAdmin:            isAdmin,
		Groups:             groupNames,
		LastLoginAt:        formatTimePtr(user.LastLoginAt),
		CreatedAt:          formatTime(user.CreatedAt),
		UpdatedAt:          formatTime(user.UpdatedAt),
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	p := currentPrincipal(r)
	if p == nil || p.IsAnonymous || p.UserID == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ctx := r.Context()

	user, err := h.db.GetUserByID(ctx, *p.UserID)
	if err != nil || user == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	if user.PasswordHash == nil {
		writeError(w, http.StatusBadRequest, "no_local_password", "user has no local password")
		return
	}

	ok, err := auth.VerifyPassword(body.CurrentPassword, *user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "current password is incorrect")
		return
	}

	if len(body.NewPassword) < 12 {
		writeError(w, http.StatusBadRequest, "validation_failed", "new password must be at least 12 characters")
		return
	}

	newHash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "password hashing failed")
		return
	}

	user.PasswordHash = &newHash

	user.MustChangePassword = false
	if err := h.db.UpdateUser(ctx, user); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "database error")
		return
	}

	writeNoContent(w)
}

func userToResponse(u *models.User) responses.UserResponse {
	return responses.UserResponse{
		ID:                 u.ID,
		Username:           u.Username,
		Email:              u.Email,
		FullName:           u.FullName,
		Disabled:           u.Disabled,
		MustChangePassword: u.MustChangePassword,
		LastLoginAt:        formatTimePtr(u.LastLoginAt),
		CreatedAt:          formatTime(u.CreatedAt),
		UpdatedAt:          formatTime(u.UpdatedAt),
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
)

const (
	SessionCookieName           = "oa_session"
	defaultSessionDurationHours = 24
)

//nolint:gochecknoglobals // exported session duration, configurable at startup
var SessionDuration = defaultSessionDurationHours * time.Hour

func ConfigureSessionDuration(d time.Duration) {
	SessionDuration = d
}

func CreateSession(ctx context.Context, db database.Database, userID, ip, userAgent string) (string, *models.Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}

	cookieValue := hex.EncodeToString(raw)
	hashBytes := sha256.Sum256(raw)
	tokenHash := hex.EncodeToString(hashBytes[:])

	now := time.Now()
	expiresAt := now.Add(SessionDuration)

	session := &models.Session{
		UserID:     userID,
		TokenHash:  tokenHash,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
		LastSeenAt: now,
	}

	if ip != "" {
		// Strip optional port (RemoteAddr is "host:port") so the value is a
		// valid Postgres inet literal.
		if host, _, splitErr := net.SplitHostPort(ip); splitErr == nil {
			ip = host
		}

		session.IPAddress = &ip
	}

	if userAgent != "" {
		session.UserAgent = &userAgent
	}

	saved, err := db.CreateSession(ctx, session)
	if err != nil {
		return "", nil, fmt.Errorf("create session: %w", err)
	}

	return cookieValue, saved, nil
}

// decodeSessionCookie decodes a hex session cookie and reports whether it has
// the expected 32-byte length.
func decodeSessionCookie(cookieValue string) ([]byte, bool) {
	raw, err := hex.DecodeString(cookieValue)

	return raw, err == nil && len(raw) == 32
}

func ValidateSession(ctx context.Context, db database.Database, cookieValue string) (*models.Session, error) {
	raw, ok := decodeSessionCookie(cookieValue)
	if !ok {
		// A malformed cookie is simply an invalid session, not a request error.
		return nil, nil //nolint:nilnil // no valid session
	}

	hashBytes := sha256.Sum256(raw)
	tokenHash := hex.EncodeToString(hashBytes[:])

	session, err := db.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}

	if session == nil {
		return nil, nil //nolint:nilnil // no valid session found is not an error
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, nil //nolint:nilnil // expired session is not an error
	}

	return session, nil
}

func ExtendSession(ctx context.Context, db database.Database, sessionID string) error {
	return db.UpdateSessionLastSeen(ctx, sessionID, time.Now().Add(SessionDuration))
}

func DeleteSession(ctx context.Context, db database.Database, cookieValue string) error {
	raw, ok := decodeSessionCookie(cookieValue)
	if !ok {
		// Nothing to delete for a malformed cookie.
		return nil
	}

	hashBytes := sha256.Sum256(raw)
	tokenHash := hex.EncodeToString(hashBytes[:])

	return db.DeleteSession(ctx, tokenHash)
}

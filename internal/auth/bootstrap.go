// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

func BootstrapIfEmpty(ctx context.Context, db database.Database, log *logger.Logger) error {
	count, err := db.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	if count > 0 {
		return nil
	}

	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate bootstrap password: %w", err)
	}

	password := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(raw)

	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	mustChange := true
	user := &models.User{
		Username:           "admin",
		PasswordHash:       &hash,
		MustChangePassword: mustChange,
	}

	created, err := db.CreateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	// Bind the bootstrap admin to the seeded admin-all permission target so the
	// account actually has manage rights on /api/v1/* and on every repository.
	// Without this assignment the RBAC middleware denies every request after
	// the password change, leaving a freshly bootstrapped instance unusable.
	if target, terr := db.GetPermissionTargetByName(ctx, "admin-all"); terr == nil && target != nil {
		assignErr := db.CreatePermissionAssignment(ctx, &models.PermissionAssignment{
			PermissionTargetID: target.ID,
			PrincipalType:      "user",
			PrincipalID:        created.ID,
		})
		if assignErr != nil {
			log.Warnw("bootstrap: could not assign admin-all to admin user", "error", assignErr)
		}
	} else if terr != nil {
		log.Warnw("bootstrap: admin-all target lookup failed", "error", terr)
	}

	fmt.Fprintf(os.Stderr,
		"\n===== OPENARTIFACT BOOTSTRAP =====\n root password: %s\n Change immediately at /api/v1/auth/change-password\n===================================\n",
		password,
	)

	log.Infow("Bootstrap completed: admin user created", "username", "admin")

	return nil
}

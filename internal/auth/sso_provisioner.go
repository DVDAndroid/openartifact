// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

type SSOClaims struct {
	ExternalID   string
	ProviderSlug string
	Username     string
	Email        string
	FullName     string
	Groups       []string
}

type Provisioner struct {
	db  database.Database
	log *logger.Logger
}

func NewProvisioner(db database.Database, log *logger.Logger) *Provisioner {
	return &Provisioner{db: db, log: log}
}

var ErrUserNotProvisioned = errors.New("user not provisioned")
var ErrAccountDisabled = errors.New("account disabled")
var ErrEmailAlreadyInUse = errors.New("email already in use")
var ErrEmailLinkAmbiguous = errors.New("multiple users share email")

//nolint:gocyclo,cyclop // protocol handler: branching mirrors spec state machine
func (p *Provisioner) Provision(ctx context.Context, provider *models.SSOProvider, claims SSOClaims) (*models.User, error) {
	user, err := p.db.GetUserByExternalID(ctx, claims.ExternalID, claims.ProviderSlug)
	if err != nil {
		return nil, fmt.Errorf("sso provision get by external id: %w", err)
	}

	if user == nil && provider.LinkByEmail && claims.Email != "" {
		existing, err := p.db.GetUserByEmail(ctx, claims.Email)
		if err != nil {
			return nil, fmt.Errorf("sso provision get by email: %w", err)
		}

		if existing != nil {
			if existing.Disabled {
				return nil, ErrAccountDisabled
			}

			if err := p.db.UpdateUserSSO(ctx, existing.ID, claims.ExternalID, claims.ProviderSlug); err != nil {
				return nil, fmt.Errorf("sso provision link by email: %w", err)
			}

			user = existing
		}
	}

	// Pre-check: when link_by_email is off, surface a clean error rather than hitting UNIQUE(email) at INSERT.
	if user == nil && !provider.LinkByEmail && claims.Email != "" {
		existing, err := p.db.GetUserByEmail(ctx, claims.Email)
		if err != nil {
			return nil, fmt.Errorf("sso provision email conflict check: %w", err)
		}

		if existing != nil {
			return nil, ErrEmailAlreadyInUse
		}
	}

	if user == nil {
		if !provider.AutoProvisionUsers {
			return nil, ErrUserNotProvisioned
		}

		username := claims.Username
		if username == "" && claims.Email != "" {
			// Fall back to email local-part when IdP doesn't provide preferred_username
			if at := strings.Index(claims.Email, "@"); at > 0 {
				username = claims.Email[:at]
			} else {
				username = claims.Email
			}
		}

		if username == "" {
			username = claims.ExternalID // last resort: use sub claim
		}

		newUser := &models.User{
			Username:           username,
			Email:              ptrIfNonEmpty(claims.Email),
			FullName:           ptrIfNonEmpty(claims.FullName),
			Disabled:           false,
			MustChangePassword: false,
			ExternalID:         &claims.ExternalID,
			ProviderSlug:       &claims.ProviderSlug,
			SSOManagedGroups:   true,
		}

		created, err := p.db.CreateUser(ctx, newUser)
		if err != nil {
			return nil, fmt.Errorf("sso provision create user: %w", err)
		}

		user = created
	}

	if user == nil {
		return nil, ErrUserNotProvisioned
	}

	if user.Disabled {
		return nil, ErrAccountDisabled
	}

	if claims.Email != "" {
		user.Email = ptrIfNonEmpty(claims.Email)
	}

	if claims.FullName != "" {
		user.FullName = ptrIfNonEmpty(claims.FullName)
	}

	user.LastLoginAt = ptrTimeNow()
	if err := p.db.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("sso provision update user: %w", err)
	}

	// Group sync failure is non-fatal: a transient error must not block an otherwise valid login.
	if err := p.SyncSSOGroupsFromClaims(ctx, user.ID, provider, claims.Groups); err != nil {
		p.log.Warnw("sso group sync failed; login continues", "user_id", user.ID, "error", err)
	}

	// Permission sync failure is non-fatal
	if err := p.SyncSSOPermissionsFromGroups(ctx, provider, claims.Groups); err != nil {
		p.log.Warnw("sso permission sync failed; login continues", "user_id", user.ID, "error", err)
	}

	return user, nil
}

func (p *Provisioner) SyncSSOGroupsFromClaims(ctx context.Context, userID string, provider *models.SSOProvider, idpGroups []string) error {
	groupIDs := make([]string, 0, len(idpGroups))
	for _, idpGroup := range idpGroups {
		localName, ok := provider.GroupMapping[idpGroup]
		if !ok {
			continue
		}

		grp, err := p.db.GetOrCreateGroup(ctx, localName)
		if err != nil {
			return fmt.Errorf("sso group get or create: %w", err)
		}
		if grp == nil {
			continue
		}

		groupIDs = append(groupIDs, grp.ID)
	}

	return p.db.SyncSSOGroups(ctx, userID, groupIDs)
}

func (p *Provisioner) SyncSSOPermissionsFromGroups(ctx context.Context, provider *models.SSOProvider, idpGroups []string) error {
	if len(provider.PermissionMapping) == 0 {
		return nil
	}
	// For each mapped group, resolve permission target and sync
	for localGroupName, permTargetName := range provider.PermissionMapping {
		// Check if this group is in the current IdP groups (via group_mapping)
		// Only assign permissions for groups the user currently has
		inCurrentGroups := false

		for _, idpGroup := range idpGroups {
			if mapped, ok := provider.GroupMapping[idpGroup]; ok && mapped == localGroupName {
				inCurrentGroups = true
				break
			}
		}

		grp, err := p.db.GetGroupByName(ctx, localGroupName)
		if err != nil || grp == nil {
			continue // group doesn't exist yet, skip
		}

		target, err := p.db.GetPermissionTargetByName(ctx, permTargetName)
		if err != nil || target == nil {
			p.log.Warnw("sso permission mapping references unknown target",
				"provider_slug", provider.Slug,
				"group", localGroupName,
				"target", permTargetName)

			continue
		}

		if inCurrentGroups {
			// Assign permission target to group
			if err := p.db.GetOrCreatePermissionAssignment(ctx, target.ID, "group", grp.ID); err != nil {
				return fmt.Errorf("sso permission assign: %w", err)
			}
		} else {
			// Remove permission target from group (user no longer in this IdP group)
			_ = p.db.DeletePermissionAssignment(ctx, target.ID, "group", grp.ID)
		}
	}

	return nil
}

func ptrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func ptrTimeNow() *time.Time {
	t := time.Now().UTC()
	return &t
}

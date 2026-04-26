// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type User struct {
	ID                 string
	Username           string
	Email              *string
	PasswordHash       *string
	FullName           *string
	Disabled           bool
	MustChangePassword bool
	LastLoginAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ExternalID         *string
	ProviderSlug       *string
	SSOManagedGroups   bool
}

type Group struct {
	ID          string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserGroup struct {
	UserID  string
	GroupID string
}

type Session struct {
	ID         string
	UserID     string
	TokenHash  string
	IPAddress  *string
	UserAgent  *string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type Token struct {
	ID         string
	UserID     string
	Name       string
	Kind       string
	TokenHash  string
	Scopes     []string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

type PermissionTarget struct {
	ID               string
	Name             string
	RepositoryFilter string
	PathPattern      string
	Actions          []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PermissionAssignment struct {
	ID                 string
	PermissionTargetID string
	PrincipalType      string
	PrincipalID        string
	CreatedAt          time.Time
}

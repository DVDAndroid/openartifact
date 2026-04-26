// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type Organization struct {
	ID          string
	Name        string
	DisplayName *string
	Description *string
	OwnerID     *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OrgMember struct {
	ID        string
	OrgID     string
	UserID    string
	Role      string // "owner", "admin", "member"
	CreatedAt time.Time
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"time"
)

// AuditLog mirrors the audit_logs table schema: structured columns for
// compliance-grade auditing.
type AuditLog struct {
	ID           string          `db:"id"`
	Action       string          `db:"action"`
	OrgID        *string         `db:"org_id"`
	RepositoryID *string         `db:"repository_id"`
	ActorID      *string         `db:"actor_id"`
	ActorType    *string         `db:"actor_type"`
	DetailsJSON  json.RawMessage `db:"details_json"`
	ResourceType *string         `db:"resource_type"`
	ResourceID   *string         `db:"resource_id"`
	IPAddress    *string         `db:"ip_address"`
	CreatedAt    time.Time       `db:"created_at"`
}

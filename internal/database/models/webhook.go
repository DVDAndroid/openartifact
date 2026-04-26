// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type Webhook struct {
	ID             string
	RepositoryID   string // empty = global
	Name           string
	URL            string
	SecretEnc      string // AES-256-GCM encrypted HMAC signing secret
	Enabled        bool
	Events         []string // ["artifact.uploaded", "artifact.deleted", "artifact.downloaded"]
	ContentType    string
	SSLVerify      bool
	MaxRetries     int
	TimeoutSeconds int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type WebhookDelivery struct {
	ID             string
	WebhookID      string
	EventType      string
	Payload        map[string]any
	Status         string // "pending", "success", "failed"
	ResponseStatus int
	ResponseBody   string
	AttemptCount   int
	NextRetryAt    *time.Time
	DeliveredAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// AIAnalysis stores the AI-generated analysis for an artifact.
type AIAnalysis struct {
	ID                    string
	RepositoryID          string
	ArtifactID            string // generic reference (component ID, npm version ID, etc.)
	ArtifactFormat        string // "npm", "maven", "deb", "rpm", "helm", "pypi", etc.
	ArtifactName          string
	ArtifactVersion       string
	Summary               string
	Tags                  []string // stored as JSONB
	LicenseStatus         string   // "SAFE", "REVIEW_REQUIRED", "UNKNOWN"
	LicenseName           string
	SecurityNotes         string
	QuarantineRecommended bool
	Status                string // "pending", "processing", "completed", "failed"
	ErrorMessage          string
	ModelUsed             string
	PromptTokens          int
	CompletionTokens      int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

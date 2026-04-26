// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// SecretFinding represents a single secret finding in a scan.
type SecretFinding struct {
	Type          string `json:"type"`
	Description   string `json:"description"`
	Severity      string `json:"severity"` // low, medium, high, critical
	RedactedValue string `json:"redacted_value"`
	LineHint      string `json:"line_hint"`
}

// SecretScanResult represents a secret scan result for an artifact.
type SecretScanResult struct {
	ID           string
	RepositoryID string
	ArtifactID   string
	ScanStatus   string // "pending", "completed", "failed", "clean", "flagged"
	Findings     []SecretFinding
	FindingCount int
	Quarantined  bool
	ScanError    string
	ScannedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

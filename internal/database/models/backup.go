// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type BackupJob struct {
	ID           string
	Status       string // "pending", "running", "completed", "failed"
	Type         string // "full", "metadata-only"
	StoragePath  string
	SizeBytes    int64
	ErrorMessage string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

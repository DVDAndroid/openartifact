// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type CleanupPolicy struct {
	ID                      string
	RepositoryID            string
	Name                    string
	Enabled                 bool
	KeepLastN               *int
	KeepSinceDays           *int
	KeepDownloadedSinceDays *int
	NamePattern             string
	ScheduleCron            string
	LastRunAt               *time.Time
	LastRunDeleted          *int
	LastRunError            string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

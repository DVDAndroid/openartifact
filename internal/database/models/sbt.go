// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type SBTArtifact struct {
	ID           string
	RepositoryID string
	GroupID      string
	ArtifactID   string
	Version      string
	ScalaVersion string
	Filename     string
	BlobKey      string
	SizeBytes    int64
	SHA256       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

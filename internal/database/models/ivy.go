// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type IvyArtifact struct {
	ID           string
	RepositoryID string
	Organisation string
	Module       string
	Revision     string
	ArtifactName string
	ArtifactType string
	ArtifactExt  string
	BlobKey      string
	SizeBytes    int64
	SHA256       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

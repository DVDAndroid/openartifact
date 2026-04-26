// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type CondaPackage struct {
	ID           string
	RepositoryID string
	Name         string
	Version      string
	BuildString  string
	Platform     string
	Depends      []string
	BlobKey      string
	SizeBytes    int64
	SHA256       string
	MD5          string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

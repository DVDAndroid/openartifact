// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type ConanPackage struct {
	ID             string
	RepositoryID   string
	Name           string
	Version        string
	UserChannel    string
	RecipeRevision string
	BlobKey        string
	ConanFile      string
	ConanInfo      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

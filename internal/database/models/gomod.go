// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type GoModule struct {
	ID           string
	RepositoryID string
	ModulePath   string
	Version      string
	GoModContent string
	ZipBlobKey   string
	ModBlobKey   string
	ZipSizeBytes int64
	ZipHash      string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

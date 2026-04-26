// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// PackageDelta represents a pre-computed bsdiff patch between two package versions.
// Clients that already have the "from" version can download the delta instead of
// the full package, saving significant bandwidth for incremental updates.
type PackageDelta struct {
	ID           string
	RepositoryID string
	// Source package (the version the client already has)
	FromSHA256   string
	FromFilename string
	// Target package (the version the client wants)
	ToSHA256   string
	ToFilename string
	// The delta blob (stored in BlobStore keyed by DeltaSHA256)
	DeltaSHA256 string
	DeltaSize   int64
	// Format: "deb" or "rpm"
	Format    string
	CreatedAt time.Time
}

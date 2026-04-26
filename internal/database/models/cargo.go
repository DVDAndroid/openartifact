// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"time"
)

type CargoCrate struct {
	ID            string
	RepositoryID  string
	Name          string
	Version       string
	Description   string
	Homepage      string
	Documentation string
	Repository    string
	License       string
	Keywords      []string
	Categories    []string
	Authors       []string
	Readme        string
	MetadataJSON  []byte
	BlobKey       string
	Checksum      string
	Yanked        bool
	DownloadCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

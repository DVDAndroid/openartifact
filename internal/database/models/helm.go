// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type HelmChart struct {
	ID           string
	RepositoryID string
	Name         string
	Version      string
	AppVersion   string
	Description  string
	Keywords     []string
	Home         string
	Sources      []string
	Maintainers  []map[string]string // [{name, email, url}]
	Icon         string
	APIVersion   string
	Type         string
	BlobKey      string
	Digest       string
	SizeBytes    int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

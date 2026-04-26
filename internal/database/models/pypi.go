// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type PyPIPackage struct {
	ID             string
	RepositoryID   string
	Name           string
	Version        string
	Filename       string
	PackageType    string
	PythonVersion  string
	RequiresPython string
	Summary        string
	Author         string
	AuthorEmail    string
	License        string
	Keywords       string
	Classifiers    []string
	BlobKey        string
	MD5Digest      string
	SHA256Digest   string
	SizeBytes      int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

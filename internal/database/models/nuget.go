// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

type NuGetPackage struct {
	ID                       string
	RepositoryID             string
	PackageID                string
	Version                  string
	Title                    string
	Description              string
	Summary                  string
	Authors                  string
	Owners                   string
	Tags                     string
	ProjectURL               string
	LicenseURL               string
	IconURL                  string
	RequireLicenseAcceptance bool
	IsPrerelease             bool
	MinClientVersion         string
	BlobKey                  string
	PackageHash              string
	PackageHashAlgorithm     string
	PackageSize              int64
	DownloadCount            int
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

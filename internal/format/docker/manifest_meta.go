// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package docker

import (
	"encoding/json"
	"strings"
)

// ociManifestMeta captures the subset of an OCI image manifest / index
// relevant to the Referrers API (OCI distribution-spec 1.1): the optional
// `subject` a manifest points at, and the metadata used to describe the
// manifest itself as a referrer entry (artifactType, annotations, and the
// config descriptor's mediaType, used as an artifactType fallback).
type ociManifestMeta struct {
	Subject *struct {
		Digest string `json:"digest"`
	} `json:"subject"`
	ArtifactType string            `json:"artifactType"` //nolint:tagliatelle // OCI image-spec wire format is camelCase
	Annotations  map[string]string `json:"annotations"`
	Config       *struct {
		MediaType string `json:"mediaType"` //nolint:tagliatelle // OCI image-spec wire format is camelCase
	} `json:"config"`
}

// parseOCIManifestMeta best-effort parses a manifest body for `subject` and
// referrer metadata. These fields are OPTIONAL and specific to the OCI
// image-spec; malformed or legacy (Docker v2 schema1) content simply yields
// a zero value rather than failing the caller.
func parseOCIManifestMeta(body []byte) ociManifestMeta {
	var m ociManifestMeta

	_ = json.Unmarshal(body, &m)

	return m
}

// manifestBlobKey returns the blob-store identifier for a manifest's
// content: the hex portion of its "sha256:<hex>" digest, matching the
// "sha256" metadata key manifests (and blobs) are Put under.
func manifestBlobKey(digest string) string {
	return strings.TrimPrefix(digest, "sha256:")
}

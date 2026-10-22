// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import _ "embed"

// OpenAPI is the OpenAPI fragment (paths, schemas, tag) of the management API
// registered by Register. internal/api merges it into the served spec.
//
//go:embed openapi.json
var OpenAPI []byte

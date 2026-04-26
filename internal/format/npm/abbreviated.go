// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"encoding/json"
	"fmt"

	"github.com/openartifact/openartifact/internal/database/models"
)

//nolint:gochecknoglobals // immutable lookup table
var abbreviatedFields = map[string]bool{
	"name":                 true,
	"version":              true,
	"dependencies":         true,
	"optionalDependencies": true,
	"devDependencies":      true,
	"bundleDependencies":   true,
	"peerDependencies":     true,
	"peerDependenciesMeta": true,
	"bin":                  true,
	"directories":          true,
	"dist":                 true,
	"engines":              true,
	"_hasShrinkwrap":       true,
	"deprecated":           true,
}

func BuildAbbreviatedPackument(repoName, packageName string, versions []*models.NPMVersion, distTags map[string]string, baseURL string) ([]byte, error) {
	versionMap := make(map[string]any, len(versions))

	for _, v := range versions {
		var full map[string]any
		if err := json.Unmarshal(v.ManifestJSON, &full); err != nil {
			return nil, fmt.Errorf("unmarshal manifest for %s@%s: %w", packageName, v.Version, err)
		}

		dist, _ := full["dist"].(map[string]any)
		if dist == nil {
			dist = make(map[string]any)
		}

		dist["tarball"] = fmt.Sprintf("%s/repos/%s/%s/-/%s", baseURL, repoName, packageName, v.TarballFilename)
		dist["integrity"] = v.Integrity
		dist["shasum"] = v.Shasum
		full["dist"] = dist

		slim := make(map[string]any, len(abbreviatedFields))
		for field := range abbreviatedFields {
			if val, ok := full[field]; ok {
				slim[field] = val
			}
		}

		versionMap[v.Version] = slim
	}

	packument := map[string]any{
		"_id":       packageName,
		"name":      packageName,
		"dist-tags": distTags,
		"versions":  versionMap,
	}

	return json.Marshal(packument)
}

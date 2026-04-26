// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openartifact/openartifact/internal/database/models"
)

func BuildPackument(repoName, packageName string, versions []*models.NPMVersion, distTags map[string]string, baseURL string) ([]byte, error) {
	versionMap := make(map[string]any, len(versions))
	timeMap := make(map[string]string, len(versions)+2)

	var oldest, newest time.Time

	for _, v := range versions {
		var manifest map[string]any
		if err := json.Unmarshal(v.ManifestJSON, &manifest); err != nil {
			return nil, fmt.Errorf("unmarshal manifest for %s@%s: %w", packageName, v.Version, err)
		}

		dist, _ := manifest["dist"].(map[string]any)
		if dist == nil {
			dist = make(map[string]any)
		}

		dist["tarball"] = fmt.Sprintf("%s/repos/%s/%s/-/%s", baseURL, repoName, packageName, v.TarballFilename)
		dist["integrity"] = v.Integrity
		dist["shasum"] = v.Shasum
		manifest["dist"] = dist

		versionMap[v.Version] = manifest
		timeMap[v.Version] = v.CreatedAt.UTC().Format(time.RFC3339)

		if oldest.IsZero() || v.CreatedAt.Before(oldest) {
			oldest = v.CreatedAt
		}

		if newest.IsZero() || v.CreatedAt.After(newest) {
			newest = v.CreatedAt
		}
	}

	if !oldest.IsZero() {
		timeMap["created"] = oldest.UTC().Format(time.RFC3339)
	}

	if !newest.IsZero() {
		timeMap["modified"] = newest.UTC().Format(time.RFC3339)
	}

	packument := map[string]any{
		"_id":       packageName,
		"name":      packageName,
		"dist-tags": distTags,
		"versions":  versionMap,
		"time":      timeMap,
	}

	return json.Marshal(packument)
}

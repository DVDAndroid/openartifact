// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/openartifact/openartifact/internal/database/models"
	fmtnpm "github.com/openartifact/openartifact/internal/format/npm"
)

func makeNPMVersion(version string, manifest map[string]any) *models.NPMVersion {
	raw, _ := json.Marshal(manifest)

	return &models.NPMVersion{
		ID:              "v-" + version,
		PackageID:       "pkg-1",
		Version:         version,
		ManifestJSON:    raw,
		TarballFilename: "lodash-" + version + ".tgz",
		Integrity:       "sha512-abc",
		Shasum:          "deadbeef",
		CreatedAt:       time.Now(),
	}
}

func TestBuildPackument_Shape(t *testing.T) {
	versions := []*models.NPMVersion{
		makeNPMVersion("1.0.0", map[string]any{"name": "lodash", "version": "1.0.0"}),
		makeNPMVersion("1.1.0", map[string]any{"name": "lodash", "version": "1.1.0"}),
	}
	distTags := map[string]string{"latest": "1.0.0"}

	data, err := fmtnpm.BuildPackument("myrepo", "lodash", versions, distTags, "http://example.com")
	if err != nil {
		t.Fatalf("BuildPackument error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"name", "_id", "dist-tags", "versions", "time"} {
		if _, ok := result[key]; !ok {
			t.Errorf("packument missing key %q", key)
		}
	}

	if result["name"] != "lodash" {
		t.Errorf("name = %v, want lodash", result["name"])
	}

	if result["_id"] != "lodash" {
		t.Errorf("_id = %v, want lodash", result["_id"])
	}

	vers, ok := result["versions"].(map[string]any)
	if !ok {
		t.Fatal("versions is not a map")
	}

	if len(vers) != 2 {
		t.Errorf("expected 2 versions, got %d", len(vers))
	}

	for _, v := range []string{"1.0.0", "1.1.0"} {
		vData, ok := vers[v].(map[string]any)
		if !ok {
			t.Errorf("version %s missing from packument", v)
			continue
		}

		dist, ok := vData["dist"].(map[string]any)
		if !ok {
			t.Errorf("version %s dist missing", v)
			continue
		}

		tarball, _ := dist["tarball"].(string)
		if !strings.HasPrefix(tarball, "http://example.com/repos/myrepo/lodash/-/") {
			t.Errorf("version %s tarball URL = %q, want prefix http://example.com/repos/myrepo/lodash/-/", v, tarball)
		}
	}
}

func TestBuildAbbreviatedPackument_Filtering(t *testing.T) {
	manifest := map[string]any{
		"name":         "lodash",
		"version":      "1.0.0",
		"description":  "A utility library",
		"readme":       "# Lodash\nVery long readme...",
		"keywords":     []string{"utility"},
		"dependencies": map[string]any{},
	}
	versions := []*models.NPMVersion{makeNPMVersion("1.0.0", manifest)}
	distTags := map[string]string{"latest": "1.0.0"}

	data, err := fmtnpm.BuildAbbreviatedPackument("myrepo", "lodash", versions, distTags, "http://example.com")
	if err != nil {
		t.Fatalf("BuildAbbreviatedPackument error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	vers, ok := result["versions"].(map[string]any)
	if !ok {
		t.Fatal("versions is not a map")
	}

	vData, ok := vers["1.0.0"].(map[string]any)
	if !ok {
		t.Fatal("version 1.0.0 not found")
	}

	if _, has := vData["description"]; has {
		t.Error("abbreviated packument should not include 'description'")
	}

	if _, has := vData["readme"]; has {
		t.Error("abbreviated packument should not include 'readme'")
	}

	if _, has := vData["keywords"]; has {
		t.Error("abbreviated packument should not include 'keywords'")
	}

	if _, has := vData["name"]; !has {
		t.Error("abbreviated packument should include 'name'")
	}

	if _, has := vData["version"]; !has {
		t.Error("abbreviated packument should include 'version'")
	}

	if _, has := vData["dist"]; !has {
		t.Error("abbreviated packument should include 'dist'")
	}

	if _, has := vData["dependencies"]; !has {
		t.Error("abbreviated packument should include 'dependencies'")
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package worker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/openartifact/openartifact/internal/database/models"
)

// ---- compareSemver / semverNums ----

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.0.0-alpha", "1.0.0", 0}, // pre-release stripped
		{"2.0.0+build", "2.0.0", 0}, // build metadata stripped
		{"10.0.0", "9.0.0", 1},
	}
	for _, tt := range tests {
		got := compareSemver(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// ---- isDeprecated ----

func TestIsDeprecated_False(t *testing.T) {
	if isDeprecated(nil) {
		t.Error("nil manifest should not be deprecated")
	}

	if isDeprecated([]byte(`{}`)) {
		t.Error("empty object should not be deprecated")
	}

	manifest, _ := json.Marshal(map[string]any{"name": "foo"})
	if isDeprecated(manifest) {
		t.Error("manifest without 'deprecated' should not be deprecated")
	}
}

func TestIsDeprecated_True(t *testing.T) {
	manifest, _ := json.Marshal(map[string]any{"deprecated": "use bar instead"})
	if !isDeprecated(manifest) {
		t.Error("manifest with non-empty 'deprecated' should be deprecated")
	}
}

func TestIsDeprecated_EmptyString(t *testing.T) {
	// deprecated="" means NOT deprecated
	manifest, _ := json.Marshal(map[string]any{"deprecated": ""})
	if isDeprecated(manifest) {
		t.Error("manifest with empty 'deprecated' string should not be deprecated")
	}
}

// ---- computeLatestVersion ----

func makeVersion(v string, deprecated bool) *models.NPMVersion {
	m := map[string]any{}
	if deprecated {
		m["deprecated"] = "old"
	}

	b, _ := json.Marshal(m)

	return &models.NPMVersion{Version: v, ManifestJSON: b}
}

func TestComputeLatestVersion_Empty(t *testing.T) {
	got := computeLatestVersion(nil)
	if got != "" {
		t.Errorf("expected empty string for nil input, got %q", got)
	}
}

func TestComputeLatestVersion_PicksHighest(t *testing.T) {
	versions := []*models.NPMVersion{
		makeVersion("1.0.0", false),
		makeVersion("2.0.0", false),
		makeVersion("1.5.0", false),
	}

	got := computeLatestVersion(versions)
	if got != "2.0.0" {
		t.Errorf("expected 2.0.0, got %q", got)
	}
}

func TestComputeLatestVersion_SkipsDeprecated(t *testing.T) {
	versions := []*models.NPMVersion{
		makeVersion("1.0.0", false),
		makeVersion("3.0.0", true), // deprecated — should be skipped
		makeVersion("2.0.0", false),
	}

	got := computeLatestVersion(versions)
	if got != "2.0.0" {
		t.Errorf("expected 2.0.0 (skip deprecated 3.0.0), got %q", got)
	}
}

func TestComputeLatestVersion_AllDeprecated(t *testing.T) {
	// Falls back to highest even if all are deprecated
	versions := []*models.NPMVersion{
		makeVersion("1.0.0", true),
		makeVersion("3.0.0", true),
	}

	got := computeLatestVersion(versions)
	if got != "3.0.0" {
		t.Errorf("expected 3.0.0 as fallback, got %q", got)
	}
}

// ---- shouldRunPolicy (white-box, unexported) ----

func TestShouldRunPolicy_NilLastRun(t *testing.T) {
	w := &CleanupWorker{}

	policy := &models.CleanupPolicy{LastRunAt: nil}
	if !w.shouldRunPolicy(policy) {
		t.Error("should run when LastRunAt is nil")
	}
}

func TestShouldRunPolicy_RecentRun(t *testing.T) {
	w := &CleanupWorker{}
	recent := time.Now().Add(-1 * time.Hour)

	policy := &models.CleanupPolicy{LastRunAt: &recent}
	if w.shouldRunPolicy(policy) {
		t.Error("should NOT run when last run was only 1 hour ago")
	}
}

func TestShouldRunPolicy_StaleRun(t *testing.T) {
	w := &CleanupWorker{}
	old := time.Now().Add(-25 * time.Hour)

	policy := &models.CleanupPolicy{LastRunAt: &old}
	if !w.shouldRunPolicy(policy) {
		t.Error("should run when last run was 25 hours ago")
	}
}

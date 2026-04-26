// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"testing"
)

func TestIsSnapshot(t *testing.T) {
	if !IsSnapshot("1.0-SNAPSHOT") {
		t.Error("expected 1.0-SNAPSHOT to be a snapshot")
	}

	if IsSnapshot("1.0-snapshot") {
		t.Error("1.0-snapshot should not be a snapshot (case-sensitive)")
	}

	if IsSnapshot("1.0") {
		t.Error("1.0 should not be a snapshot")
	}
}

func TestParseSnapshotTimestamp(t *testing.T) {
	ts, bn, base, err := ParseSnapshotTimestamp("app-1.0-20260426.120000-3.jar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ts != "20260426.120000" {
		t.Errorf("timestamp = %q, want %q", ts, "20260426.120000")
	}

	if bn != 3 {
		t.Errorf("buildNumber = %d, want 3", bn)
	}

	if base != "app-1.0.jar" {
		t.Errorf("baseFilename = %q, want %q", base, "app-1.0.jar")
	}
}

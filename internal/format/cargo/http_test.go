// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package cargo

import (
	"encoding/json"
	"testing"
)

// TestTransformCargoIndexDeps exercises the publish-API -> registry-index
// dependency schema translation: "version_req" -> "req", and a
// rename's original/alias names swap between name/explicit_name_in_toml and
// name/package. See https://doc.rust-lang.org/cargo/reference/registry-index.html
func TestTransformCargoIndexDeps(t *testing.T) {
	raw := `[
		{
			"name": "serde",
			"version_req": "^1.0",
			"features": ["derive"],
			"optional": true,
			"default_features": false,
			"target": "cfg(windows)",
			"kind": "dev",
			"registry": null
		},
		{
			"name": "rand",
			"explicit_name_in_toml": "random",
			"version_req": ">=0.8, <0.9"
		}
	]`

	var deps []any
	if err := json.Unmarshal([]byte(raw), &deps); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	got := transformCargoIndexDeps(deps)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	serde := got[0]
	if serde["name"] != "serde" {
		t.Errorf("serde name = %v, want serde", serde["name"])
	}

	if serde["req"] != "^1.0" {
		t.Errorf("serde req = %v, want ^1.0 (from version_req)", serde["req"])
	}

	if serde["package"] != nil {
		t.Errorf("serde package = %v, want nil (not renamed)", serde["package"])
	}

	if serde["kind"] != "dev" {
		t.Errorf("serde kind = %v, want dev", serde["kind"])
	}

	renamed := got[1]
	if renamed["name"] != "random" {
		t.Errorf("renamed name = %v, want random (the Cargo.toml alias)", renamed["name"])
	}

	if renamed["package"] != "rand" {
		t.Errorf("renamed package = %v, want rand (the original crate name)", renamed["package"])
	}

	if renamed["req"] != ">=0.8, <0.9" {
		t.Errorf("renamed req = %v, want >=0.8, <0.9", renamed["req"])
	}

	// Fields the publish payload omitted entirely must fall back to the
	// index schema's documented defaults, not be dropped.
	if v, ok := renamed["features"].([]any); !ok || len(v) != 0 {
		t.Errorf("renamed features = %#v, want empty slice default", renamed["features"])
	}

	if renamed["optional"] != false {
		t.Errorf("renamed optional = %v, want false default", renamed["optional"])
	}

	if renamed["default_features"] != true {
		t.Errorf("renamed default_features = %v, want true default", renamed["default_features"])
	}

	if renamed["kind"] != "normal" {
		t.Errorf("renamed kind = %v, want normal default", renamed["kind"])
	}
}

func TestTransformCargoIndexDepsNilOrMalformed(t *testing.T) {
	if got := transformCargoIndexDeps(nil); len(got) != 0 {
		t.Errorf("nil deps = %#v, want empty slice", got)
	}

	if got := transformCargoIndexDeps("not an array"); len(got) != 0 {
		t.Errorf("malformed deps = %#v, want empty slice", got)
	}
}

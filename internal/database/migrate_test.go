// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"testing"
)

func TestDetectNoTx(t *testing.T) {
	cases := []struct {
		filename string
		want     bool
	}{
		{"001_initial_schema.sql", false},
		{"005_add_index.no_tx.sql", true},
		{"foo.no_tx.sql", true},
		{"no_tx.sql", false},
		{"notx.sql", false},
	}
	for _, c := range cases {
		got := detectNoTx(c.filename)
		if got != c.want {
			t.Errorf("detectNoTx(%q) = %v, want %v", c.filename, got, c.want)
		}
	}
}

func TestMigratorChecksum(t *testing.T) {
	data := []byte("SELECT 1;")

	sum := migratorChecksum(data)
	if len(sum) != 64 {
		t.Errorf("checksum length = %d, want 64 hex chars", len(sum))
	}

	if migratorChecksum(data) != sum {
		t.Error("checksum is not deterministic")
	}

	if migratorChecksum([]byte("SELECT 2;")) == sum {
		t.Error("different data produced same checksum")
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"bytes"
	"testing"
)

func TestComputeIntegrity_Hello(t *testing.T) {
	integrity, sha1Hex, sha256Hex, size, err := ComputeIntegrity(bytes.NewReader([]byte("hello")))
	if err != nil {
		t.Fatalf("ComputeIntegrity error: %v", err)
	}

	wantIntegrity := "sha512-m3HSJL1i83hdltRq0+o9czGb+8KJDKra4t/3JRlnPKcjI8PZm6XBHXx6zG4UuMXaDEZjR1wuXDre9G9zvN7AQw=="
	if integrity != wantIntegrity {
		t.Errorf("integrity = %q, want %q", integrity, wantIntegrity)
	}

	wantSHA1 := "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
	if sha1Hex != wantSHA1 {
		t.Errorf("sha1Hex = %q, want %q", sha1Hex, wantSHA1)
	}

	wantSHA256 := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if sha256Hex != wantSHA256 {
		t.Errorf("sha256Hex = %q, want %q", sha256Hex, wantSHA256)
	}

	if size != 5 {
		t.Errorf("size = %d, want 5", size)
	}
}

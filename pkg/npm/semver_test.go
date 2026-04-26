// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"errors"
	"testing"
)

func TestValidateSemver_Valid(t *testing.T) {
	cases := []string{
		"1.0.0",
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-rc.1+build.1",
		"0.0.0",
		"1.2.3-beta.1",
		"1.2.3+build.456",
	}
	for _, v := range cases {
		if err := ValidateSemver(v); err != nil {
			t.Errorf("ValidateSemver(%q) unexpected error: %v", v, err)
		}
	}
}

func TestValidateSemver_Invalid(t *testing.T) {
	cases := []string{
		"1.0",
		"v1.0.0",
		"latest",
		"",
		"1.0.0.0",
	}
	for _, v := range cases {
		err := ValidateSemver(v)
		if err == nil {
			t.Errorf("ValidateSemver(%q) expected error, got nil", v)
		}

		if v != "" && !errors.Is(err, ErrInvalidSemver) {
			t.Errorf("ValidateSemver(%q) expected ErrInvalidSemver, got %v", v, err)
		}
	}
}

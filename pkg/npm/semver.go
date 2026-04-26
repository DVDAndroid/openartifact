// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"errors"
	"fmt"
	"regexp"
)

var ErrInvalidSemver = errors.New("invalid semver")

var semverRe = regexp.MustCompile(
	`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`,
)

func ValidateSemver(v string) error {
	if v == "" {
		return fmt.Errorf("%w: must not be empty", ErrInvalidSemver)
	}

	if !semverRe.MatchString(v) {
		return fmt.Errorf("%w: %q does not match semver 2.0.0", ErrInvalidSemver, v)
	}

	return nil
}

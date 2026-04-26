// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"crypto/sha1" //nolint:gosec // required by protocol specification
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"io"
)

func ComputeIntegrity(r io.Reader) (string, string, string, int64, error) {
	h1 := sha1.New() //nolint:gosec // required by protocol specification
	h256 := sha256.New()
	h512 := sha512.New()

	mw := io.MultiWriter(h1, h256, h512)

	sizeBytes, err := io.Copy(mw, r)
	if err != nil {
		return "", "", "", 0, err
	}

	integrity := "sha512-" + base64.StdEncoding.EncodeToString(h512.Sum(nil))
	sha1Hex := hex.EncodeToString(h1.Sum(nil))
	sha256Hex := hex.EncodeToString(h256.Sum(nil))

	return integrity, sha1Hex, sha256Hex, sizeBytes, nil
}

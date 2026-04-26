// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"crypto/md5"  //nolint:gosec // required by protocol specification
	"crypto/sha1" //nolint:gosec // required by protocol specification
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io"
)

func ComputeChecksums(r io.Reader) (string, string, string, string, error) {
	hMD5 := md5.New()   //nolint:gosec // required by protocol specification
	hSHA1 := sha1.New() //nolint:gosec // required by protocol specification
	hSHA256 := sha256.New()
	hSHA512 := sha512.New()

	mw := io.MultiWriter(hMD5, hSHA1, hSHA256, hSHA512)
	if _, err := io.Copy(mw, r); err != nil {
		return "", "", "", "", err
	}

	return hex.EncodeToString(hMD5.Sum(nil)),
		hex.EncodeToString(hSHA1.Sum(nil)),
		hex.EncodeToString(hSHA256.Sum(nil)),
		hex.EncodeToString(hSHA512.Sum(nil)),
		nil
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1" //nolint:gosec // required by protocol specification
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	maxTarballSize     = 200 << 20 // 200 MiB
	maxPackageJSONSize = 4 << 20   // 4 MiB
)

var (
	ErrTarballTooLarge     = errors.New("tarball exceeds max size")
	ErrPackageJSONTooLarge = errors.New("package.json exceeds max size")
)

// readAllLimited reads at most max+1 bytes from r and errors if the
// result exceeds max, without ever allocating more than max+1 bytes.
func readAllLimited(r io.Reader, limit int64, tooLarge error) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: max %d bytes", tooLarge, limit)
	}

	return data, nil
}

func ParseTarball(r io.Reader) (map[string]any, string, string, error) {
	raw, err := readAllLimited(r, maxTarballSize, ErrTarballTooLarge)
	if err != nil {
		return nil, "", "", fmt.Errorf("reading tarball: %w", err)
	}

	h512 := sha512.Sum512(raw)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(h512[:])

	h1 := sha1.Sum(raw) //nolint:gosec // required by protocol specification
	shasum := hex.EncodeToString(h1[:])

	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, "", "", fmt.Errorf("gzip open: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, "", "", fmt.Errorf("tar read: %w", err)
		}

		if hdr.Name == "package/package.json" {
			data, err := readAllLimited(tr, maxPackageJSONSize, ErrPackageJSONTooLarge)
			if err != nil {
				return nil, "", "", fmt.Errorf("read package.json: %w", err)
			}

			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				return nil, "", "", fmt.Errorf("decode package.json: %w", err)
			}

			return m, integrity, shasum, nil
		}
	}

	return nil, "", "", errors.New("tarball missing package/package.json") //nolint:err113 // static error
}

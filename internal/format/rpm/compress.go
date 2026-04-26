// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/ulikunitz/xz"
)

type CompressedMetadata struct {
	GzipData []byte
	XZData   []byte
	GzSHA256 string
	XZSHA256 string
	GzSize   int64
	XZSize   int64
	OpenSize int64
}

func CompressMetadata(xmlData []byte) (*CompressedMetadata, error) {
	if len(xmlData) == 0 {
		return nil, errors.New("rpm: empty xml data") //nolint:err113 // static error
	}

	gzData, err := compressGzip(xmlData)
	if err != nil {
		return nil, fmt.Errorf("rpm: gzip compression: %w", err)
	}

	xzData, err := compressXZ(xmlData)
	if err != nil {
		return nil, fmt.Errorf("rpm: xz compression: %w", err)
	}

	gzHash := sha256.Sum256(gzData)
	xzHash := sha256.Sum256(xzData)

	return &CompressedMetadata{
		GzipData: gzData,
		XZData:   xzData,
		GzSHA256: hex.EncodeToString(gzHash[:]),
		XZSHA256: hex.EncodeToString(xzHash[:]),
		GzSize:   int64(len(gzData)),
		XZSize:   int64(len(xzData)),
		OpenSize: int64(len(xmlData)),
	}, nil
}

func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func compressXZ(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	w, err := xz.NewWriter(&buf)
	if err != nil {
		return nil, err
	}

	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

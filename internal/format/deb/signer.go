// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/openartifact/openartifact/pkg/gpg"
)

type ReleaseSigner struct {
	signer gpg.Signer
}

func NewReleaseSigner(signer gpg.Signer) *ReleaseSigner {
	return &ReleaseSigner{signer: signer}
}

type ReleaseContent struct {
	Origin        string
	Label         string
	Suite         string
	Version       string
	Codename      string
	Date          time.Time
	ValidUntil    time.Time
	Architectures []string
	Components    []string
	Description   string
	SHA256        []ReleaseChecksum
	SHA512        []ReleaseChecksum
}

type ReleaseChecksum struct {
	Hash     string
	Size     int64
	Filename string
}

func (rs *ReleaseSigner) GenerateRelease(content ReleaseContent) ([]byte, error) {
	var buf bytes.Buffer

	if content.Origin != "" {
		fmt.Fprintf(&buf, "Origin: %s\n", content.Origin)
	}

	if content.Label != "" {
		fmt.Fprintf(&buf, "Label: %s\n", content.Label)
	}

	if content.Suite != "" {
		fmt.Fprintf(&buf, "Suite: %s\n", content.Suite)
	}

	if content.Version != "" {
		fmt.Fprintf(&buf, "Version: %s\n", content.Version)
	}

	if content.Codename != "" {
		fmt.Fprintf(&buf, "Codename: %s\n", content.Codename)
	}

	date := content.Date
	if date.IsZero() {
		date = time.Now().UTC()
	}

	fmt.Fprintf(&buf, "Date: %s\n", date.Format(time.RFC1123))

	if !content.ValidUntil.IsZero() {
		fmt.Fprintf(&buf, "Valid-Until: %s\n", content.ValidUntil.Format(time.RFC1123))
	}

	if len(content.Architectures) > 0 {
		sort.Strings(content.Architectures)
		fmt.Fprintf(&buf, "Architectures: %s\n", strings.Join(content.Architectures, " "))
	}

	if len(content.Components) > 0 {
		sort.Strings(content.Components)
		fmt.Fprintf(&buf, "Components: %s\n", strings.Join(content.Components, " "))
	}

	if content.Description != "" {
		fmt.Fprintf(&buf, "Description: %s\n", content.Description)
	}

	if len(content.SHA256) > 0 {
		fmt.Fprintf(&buf, "SHA256:\n")

		for _, cs := range content.SHA256 {
			fmt.Fprintf(&buf, " %s %d %s\n", cs.Hash, cs.Size, cs.Filename)
		}
	}

	if len(content.SHA512) > 0 {
		fmt.Fprintf(&buf, "SHA512:\n")

		for _, cs := range content.SHA512 {
			fmt.Fprintf(&buf, " %s %d %s\n", cs.Hash, cs.Size, cs.Filename)
		}
	}

	return buf.Bytes(), nil
}

func (rs *ReleaseSigner) SignRelease(releaseData []byte) (*SignedRelease, error) {
	if rs.signer == nil {
		return nil, errors.New("no signer configured") //nolint:err113 // static error
	}

	detachedSig, err := rs.signer.DetachedSign(bytes.NewReader(releaseData))
	if err != nil {
		return nil, fmt.Errorf("failed to create detached signature: %w", err)
	}

	inReleaseReader, err := rs.signer.CleartextSign(bytes.NewReader(releaseData))
	if err != nil {
		return nil, fmt.Errorf("failed to create cleartext signature: %w", err)
	}

	inReleaseData, err := io.ReadAll(inReleaseReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read InRelease data: %w", err)
	}

	return &SignedRelease{
		Release:     releaseData,
		InRelease:   inReleaseData,
		ReleaseGPG:  detachedSig,
		Fingerprint: rs.signer.KeyFingerprint(),
	}, nil
}

type SignedRelease struct {
	Release     []byte
	InRelease   []byte
	ReleaseGPG  []byte
	Fingerprint string
}

func (rs *ReleaseSigner) GetPublicKey() ([]byte, error) {
	if rs.signer == nil {
		return nil, errors.New("no signer configured") //nolint:err113 // static error
	}

	return rs.signer.PublicKey()
}

func (rs *ReleaseSigner) IsConfigured() bool {
	return rs.signer != nil
}

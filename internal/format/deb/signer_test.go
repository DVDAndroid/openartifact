// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/openartifact/openartifact/pkg/gpg"
)

func TestReleaseSigner_GenerateRelease(t *testing.T) {
	signer := NewReleaseSigner(nil)

	content := ReleaseContent{
		Origin:        "OpenArtifact",
		Label:         "Test Repository",
		Suite:         "stable",
		Version:       "1.0",
		Codename:      "bookworm",
		Date:          time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Architectures: []string{"amd64", "arm64", "all"},
		Components:    []string{"main", "contrib"},
		Description:   "Test Debian Repository",
		SHA256: []ReleaseChecksum{
			{Hash: "abc123", Size: 1024, Filename: "main/binary-amd64/Packages"},
		},
	}

	data, err := signer.GenerateRelease(content)
	if err != nil {
		t.Fatalf("GenerateRelease failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("GenerateRelease returned empty data")
	}

	result := string(data)
	if !strings.Contains(result, "Origin: OpenArtifact") {
		t.Error("Missing Origin field")
	}

	if !strings.Contains(result, "Suite: stable") {
		t.Error("Missing Suite field")
	}

	if !strings.Contains(result, "Architectures: all amd64 arm64") {
		t.Error("Missing or incorrect Architectures field")
	}

	if !strings.Contains(result, "SHA256:") {
		t.Error("Missing SHA256 section")
	}
}

func TestReleaseSigner_SignRelease(t *testing.T) {
	entity, err := gpg.GenerateKey("Test Key", "", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	var keyBuf bytes.Buffer

	w, _ := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	_ = entity.SerializePrivate(w, nil)
	_ = w.Close()

	gpgSigner, err := gpg.NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)
	if err != nil {
		t.Fatalf("NewLocalSigner failed: %v", err)
	}

	signer := NewReleaseSigner(gpgSigner)

	releaseData := []byte("Origin: Test\nSuite: stable\n")

	signed, err := signer.SignRelease(releaseData)
	if err != nil {
		t.Fatalf("SignRelease failed: %v", err)
	}

	if !bytes.Equal(signed.Release, releaseData) {
		t.Error("Release data was modified")
	}

	if len(signed.InRelease) == 0 {
		t.Error("InRelease is empty")
	}

	if len(signed.ReleaseGPG) == 0 {
		t.Error("ReleaseGPG is empty")
	}

	if signed.Fingerprint == "" {
		t.Error("Fingerprint is empty")
	}
}

func TestReleaseSigner_GetPublicKey(t *testing.T) {
	entity, err := gpg.GenerateKey("Test Key", "", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	var keyBuf bytes.Buffer

	w, _ := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	_ = entity.SerializePrivate(w, nil)
	_ = w.Close()

	gpgSigner, err := gpg.NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)
	if err != nil {
		t.Fatalf("NewLocalSigner failed: %v", err)
	}

	signer := NewReleaseSigner(gpgSigner)

	pubKey, err := signer.GetPublicKey()
	if err != nil {
		t.Fatalf("GetPublicKey failed: %v", err)
	}

	if len(pubKey) == 0 {
		t.Fatal("GetPublicKey returned empty key")
	}
}

func TestReleaseSigner_IsConfigured(t *testing.T) {
	signer1 := NewReleaseSigner(nil)
	if signer1.IsConfigured() {
		t.Error("Expected IsConfigured to be false when signer is nil")
	}

	entity, _ := gpg.GenerateKey("Test Key", "", "test@example.com")

	var keyBuf bytes.Buffer

	w, _ := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	_ = entity.SerializePrivate(w, nil)
	_ = w.Close()

	gpgSigner, _ := gpg.NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)

	signer2 := NewReleaseSigner(gpgSigner)
	if !signer2.IsConfigured() {
		t.Error("Expected IsConfigured to be true when signer is set")
	}
}

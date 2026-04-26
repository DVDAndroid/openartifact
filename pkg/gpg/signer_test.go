// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package gpg

import (
	"bytes"
	"io"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

func TestGenerateKey(t *testing.T) {
	entity, err := GenerateKey("Test User", "Test Comment", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if entity == nil {
		t.Fatal("GenerateKey returned nil entity")
	}

	if entity.PrimaryKey == nil {
		t.Fatal("Generated key has no primary key")
	}

	if len(entity.Identities) == 0 {
		t.Fatal("Generated key has no identities")
	}
}

func TestLocalSigner(t *testing.T) {
	entity, err := GenerateKey("Test User", "", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	var keyBuf bytes.Buffer

	w, err := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatalf("Failed to create armor encoder: %v", err)
	}

	err = entity.SerializePrivate(w, nil)
	if err != nil {
		t.Fatalf("Failed to serialize private key: %v", err)
	}

	_ = w.Close()

	signer, err := NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)
	if err != nil {
		t.Fatalf("NewLocalSigner failed: %v", err)
	}

	if signer.KeyFingerprint() == "" {
		t.Fatal("KeyFingerprint returned empty string")
	}

	if signer.KeyID() == "" {
		t.Fatal("KeyID returned empty string")
	}
}

func TestDetachedSign(t *testing.T) {
	entity, err := GenerateKey("Test User", "", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	var keyBuf bytes.Buffer

	w, err := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatalf("Failed to create armor encoder: %v", err)
	}

	err = entity.SerializePrivate(w, nil)
	if err != nil {
		t.Fatalf("Failed to serialize private key: %v", err)
	}

	_ = w.Close()

	signer, err := NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)
	if err != nil {
		t.Fatalf("NewLocalSigner failed: %v", err)
	}

	data := bytes.NewReader([]byte("test data to sign"))

	sig, err := signer.DetachedSign(data)
	if err != nil {
		t.Fatalf("DetachedSign failed: %v", err)
	}

	if len(sig) == 0 {
		t.Fatal("DetachedSign returned empty signature")
	}

	if !bytes.Contains(sig, []byte("-----BEGIN PGP SIGNATURE-----")) {
		t.Fatal("Signature missing PGP armor header")
	}
}

func TestPublicKey(t *testing.T) {
	entity, err := GenerateKey("Test User", "", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	var keyBuf bytes.Buffer

	w, err := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatalf("Failed to create armor encoder: %v", err)
	}

	err = entity.SerializePrivate(w, nil)
	if err != nil {
		t.Fatalf("Failed to serialize private key: %v", err)
	}

	_ = w.Close()

	signer, err := NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)
	if err != nil {
		t.Fatalf("NewLocalSigner failed: %v", err)
	}

	pubKey, err := signer.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey failed: %v", err)
	}

	if len(pubKey) == 0 {
		t.Fatal("PublicKey returned empty key")
	}

	if !bytes.Contains(pubKey, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----")) {
		t.Fatal("Public key missing PGP armor header")
	}
}

func TestCleartextSign(t *testing.T) {
	entity, err := GenerateKey("Test User", "", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	var keyBuf bytes.Buffer

	w, err := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatalf("Failed to create armor encoder: %v", err)
	}

	err = entity.SerializePrivate(w, nil)
	if err != nil {
		t.Fatalf("Failed to serialize private key: %v", err)
	}

	_ = w.Close()

	signer, err := NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)
	if err != nil {
		t.Fatalf("NewLocalSigner failed: %v", err)
	}

	data := bytes.NewReader([]byte("test data to sign"))

	signed, err := signer.CleartextSign(data)
	if err != nil {
		t.Fatalf("CleartextSign failed: %v", err)
	}

	signedData, err := io.ReadAll(signed)
	if err != nil {
		t.Fatalf("Failed to read signed data: %v", err)
	}

	if len(signedData) == 0 {
		t.Fatal("CleartextSign returned empty data")
	}

	if !bytes.Contains(signedData, []byte("-----BEGIN PGP SIGNED MESSAGE-----")) {
		t.Fatal("Signed message missing cleartext header")
	}

	if !bytes.Contains(signedData, []byte("-----BEGIN PGP SIGNATURE-----")) {
		t.Fatal("Signed message missing signature block")
	}

	if !bytes.Contains(signedData, []byte("test data to sign")) {
		t.Fatal("Signed message missing original content")
	}
}

func TestExportPrivateKey(t *testing.T) {
	entity, err := GenerateKey("Test User", "", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	var keyBuf bytes.Buffer

	w, err := armor.Encode(&keyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatalf("Failed to create armor encoder: %v", err)
	}

	err = entity.SerializePrivate(w, nil)
	if err != nil {
		t.Fatalf("Failed to serialize private key: %v", err)
	}

	_ = w.Close()

	signer, err := NewLocalSigner(bytes.NewReader(keyBuf.Bytes()), nil)
	if err != nil {
		t.Fatalf("NewLocalSigner failed: %v", err)
	}

	exported, err := signer.ExportPrivateKey()
	if err != nil {
		t.Fatalf("ExportPrivateKey failed: %v", err)
	}

	if len(exported) == 0 {
		t.Fatal("ExportPrivateKey returned empty key")
	}

	if !bytes.Contains(exported, []byte("-----BEGIN PGP PRIVATE KEY BLOCK-----")) {
		t.Fatal("Exported key missing PGP armor header")
	}
}

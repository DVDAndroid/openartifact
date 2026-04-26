// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

func makeAtt(content []byte, ct string, length any) map[string]any {
	return map[string]any{
		"content_type": ct,
		"data":         base64.StdEncoding.EncodeToString(content),
		"length":       length,
	}
}

func TestDecodeAttachment_Valid(t *testing.T) {
	payload := []byte("hello tarball bytes")
	att := makeAtt(payload, "application/octet-stream", float64(len(payload)))

	data, ct, err := DecodeAttachment(att)
	if err != nil {
		t.Fatalf("DecodeAttachment error: %v", err)
	}

	if ct != "application/octet-stream" {
		t.Errorf("content_type = %q, want application/octet-stream", ct)
	}

	if !bytes.Equal(data, payload) {
		t.Errorf("decoded data mismatch")
	}
}

func TestDecodeAttachment_LengthMismatch(t *testing.T) {
	payload := []byte("hello")
	att := makeAtt(payload, "application/octet-stream", float64(99))

	_, _, err := DecodeAttachment(att)
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}

	if !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("expected ErrInvalidAttachment, got %v", err)
	}
}

func TestDecodeAttachment_WrongContentType(t *testing.T) {
	payload := []byte("hello")
	att := makeAtt(payload, "text/plain", float64(len(payload)))

	_, _, err := DecodeAttachment(att)
	if err == nil {
		t.Fatal("expected error for wrong content_type")
	}

	if !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("expected ErrInvalidAttachment, got %v", err)
	}
}

func TestDecodeAttachment_MissingContentType(t *testing.T) {
	att := map[string]any{
		"data":   base64.StdEncoding.EncodeToString([]byte("x")),
		"length": float64(1),
	}

	_, _, err := DecodeAttachment(att)
	if err == nil {
		t.Fatal("expected error for missing content_type")
	}

	if !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("expected ErrInvalidAttachment, got %v", err)
	}
}

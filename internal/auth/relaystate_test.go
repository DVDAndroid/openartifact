// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"errors"
	"testing"
)

func TestSignVerifyRelayState_roundtrip(t *testing.T) {
	key := "test-secret-key"
	redirect := "https://example.com/dashboard"

	encoded := SignRelayState(redirect, key)

	got, err := VerifyRelayState(encoded, key)
	if err != nil {
		t.Fatalf("VerifyRelayState returned error: %v", err)
	}

	if got != redirect {
		t.Fatalf("expected %q, got %q", redirect, got)
	}
}

func TestVerifyRelayState_tamper(t *testing.T) {
	key := "test-secret-key"
	redirect := "https://example.com/dashboard"

	encoded := SignRelayState(redirect, key)

	_, err := VerifyRelayState(encoded+"X", key)
	if !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestVerifyRelayState_wrongKey(t *testing.T) {
	redirect := "https://example.com/page"
	encoded := SignRelayState(redirect, "key-a")

	_, err := VerifyRelayState(encoded, "key-b")
	if !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

func TestVerifyRelayState_invalidBase64(t *testing.T) {
	_, err := VerifyRelayState("!!!not-base64!!!", "key")
	if !errors.Is(err, ErrInvalidRelayState) {
		t.Fatalf("expected ErrInvalidRelayState, got %v", err)
	}
}

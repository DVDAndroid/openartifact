// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package conda

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCondaHandler_Meta(t *testing.T) {
	h := NewCondaHandler()
	if h.Name() != "conda" {
		t.Errorf("Name() = %q, want conda", h.Name())
	}

	if h.GetFileExtension() != ".tar.bz2" {
		t.Errorf("GetFileExtension() = %q, want .tar.bz2", h.GetFileExtension())
	}
}

func TestCondaHandler_Validate(t *testing.T) {
	h := NewCondaHandler()
	ctx := context.Background()

	t.Run("valid bzip2 magic", func(t *testing.T) {
		// BZ magic: 0x42 0x5a
		data := []byte{0x42, 0x5a, 0x68, 0x39}
		if err := h.Validate(ctx, bytes.NewReader(data)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid: not bzip2", func(t *testing.T) {
		if err := h.Validate(ctx, strings.NewReader("notbzip2")); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("invalid: empty", func(t *testing.T) {
		if err := h.Validate(ctx, bytes.NewReader(nil)); err == nil {
			t.Error("expected error for empty reader")
		}
	})
}

func TestCondaHandler_Parse(t *testing.T) {
	h := NewCondaHandler()
	ctx := context.Background()

	result, err := h.Parse(ctx, strings.NewReader("anything"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["format"] != "conda" {
		t.Errorf("format = %v, want conda", result["format"])
	}
}

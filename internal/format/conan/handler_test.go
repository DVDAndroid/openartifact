// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package conan

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestConanHandler_Meta(t *testing.T) {
	h := NewConanHandler()
	if h.Name() != "conan" {
		t.Errorf("Name() = %q, want conan", h.Name())
	}

	if h.GetFileExtension() != ".tar.gz" {
		t.Errorf("GetFileExtension() = %q, want .tar.gz", h.GetFileExtension())
	}
}

func TestConanHandler_Validate(t *testing.T) {
	h := NewConanHandler()
	ctx := context.Background()

	t.Run("valid gzip magic", func(t *testing.T) {
		data := []byte{0x1f, 0x8b, 0x08, 0x00}
		if err := h.Validate(ctx, bytes.NewReader(data)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid: not gzip", func(t *testing.T) {
		if err := h.Validate(ctx, strings.NewReader("notgzip")); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("invalid: empty", func(t *testing.T) {
		if err := h.Validate(ctx, bytes.NewReader(nil)); err == nil {
			t.Error("expected error for empty reader")
		}
	})
}

func TestConanHandler_Parse(t *testing.T) {
	h := NewConanHandler()
	ctx := context.Background()

	result, err := h.Parse(ctx, strings.NewReader("anything"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["format"] != "conan" {
		t.Errorf("format = %v, want conan", result["format"])
	}
}

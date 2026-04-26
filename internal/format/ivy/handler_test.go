// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package ivy

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestIvyHandler_Meta(t *testing.T) {
	h := NewIvyHandler()
	if h.Name() != "ivy" {
		t.Errorf("Name() = %q, want ivy", h.Name())
	}

	if h.GetFileExtension() != ".jar" {
		t.Errorf("GetFileExtension() = %q, want .jar", h.GetFileExtension())
	}
}

func TestIvyHandler_Validate(t *testing.T) {
	h := NewIvyHandler()
	ctx := context.Background()

	t.Run("valid JAR magic bytes", func(t *testing.T) {
		// ZIP/JAR magic: 50 4b 03 04
		data := []byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0x00}
		if err := h.Validate(ctx, bytes.NewReader(data)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid: not JAR", func(t *testing.T) {
		if err := h.Validate(ctx, strings.NewReader("NOTAJAR")); err == nil {
			t.Error("expected error for invalid JAR")
		}
	})

	t.Run("invalid: empty reader", func(t *testing.T) {
		if err := h.Validate(ctx, bytes.NewReader(nil)); err == nil {
			t.Error("expected error for empty reader")
		}
	})
}

func TestIvyHandler_Parse(t *testing.T) {
	h := NewIvyHandler()
	ctx := context.Background()

	result, err := h.Parse(ctx, strings.NewReader("anything"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["format"] != "ivy" {
		t.Errorf("format = %v, want ivy", result["format"])
	}
}

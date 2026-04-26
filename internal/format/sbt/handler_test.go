// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package sbt

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSBTHandler_Meta(t *testing.T) {
	h := NewSBTHandler()
	if h.Name() != "sbt" {
		t.Errorf("Name() = %q, want sbt", h.Name())
	}

	if h.GetFileExtension() != ".jar" {
		t.Errorf("GetFileExtension() = %q, want .jar", h.GetFileExtension())
	}
}

func TestSBTHandler_Validate(t *testing.T) {
	h := NewSBTHandler()
	ctx := context.Background()

	t.Run("valid JAR magic", func(t *testing.T) {
		data := []byte{0x50, 0x4b, 0x03, 0x04, 0x00}
		if err := h.Validate(ctx, bytes.NewReader(data)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid: not JAR", func(t *testing.T) {
		if err := h.Validate(ctx, strings.NewReader("notajar")); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("invalid: empty", func(t *testing.T) {
		if err := h.Validate(ctx, bytes.NewReader(nil)); err == nil {
			t.Error("expected error for empty reader")
		}
	})
}

func TestSBTHandler_Parse(t *testing.T) {
	h := NewSBTHandler()
	ctx := context.Background()

	result, err := h.Parse(ctx, strings.NewReader("anything"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["format"] != "sbt" {
		t.Errorf("format = %v, want sbt", result["format"])
	}
}

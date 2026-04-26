// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package cargo

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func createCratePayload(t *testing.T, metadata map[string]any) []byte {
	t.Helper()

	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(metaJSON))); err != nil {
		t.Fatalf("binary write: %v", err)
	}

	buf.Write(metaJSON)
	// Append fake crate content (tar.gz body would go here)
	buf.WriteString("fake crate body")

	return buf.Bytes()
}

func TestCargoHandler_Meta(t *testing.T) {
	h := NewCargoHandler()
	if h.Name() != "cargo" {
		t.Errorf("Name() = %q, want cargo", h.Name())
	}

	if h.GetFileExtension() != ".crate" {
		t.Errorf("GetFileExtension() = %q, want .crate", h.GetFileExtension())
	}
}

func TestCargoHandler_Validate(t *testing.T) {
	h := NewCargoHandler()
	ctx := context.Background()

	t.Run("valid crate", func(t *testing.T) {
		payload := createCratePayload(t, map[string]any{
			"name": "mylib",
			"vers": "1.0.0",
		})
		if err := h.Validate(ctx, bytes.NewReader(payload)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid: empty reader", func(t *testing.T) {
		if err := h.Validate(ctx, bytes.NewReader(nil)); err == nil {
			t.Error("expected error for empty reader")
		}
	})

	t.Run("invalid: missing name field", func(t *testing.T) {
		payload := createCratePayload(t, map[string]any{"vers": "1.0.0"})
		if err := h.Validate(ctx, bytes.NewReader(payload)); err == nil {
			t.Error("expected error for missing name")
		}
	})

	t.Run("invalid: missing vers field", func(t *testing.T) {
		payload := createCratePayload(t, map[string]any{"name": "mylib"})
		if err := h.Validate(ctx, bytes.NewReader(payload)); err == nil {
			t.Error("expected error for missing vers")
		}
	})
}

func TestCargoHandler_Parse(t *testing.T) {
	h := NewCargoHandler()
	ctx := context.Background()

	meta := map[string]any{
		"name": "serde",
		"vers": "1.0.195",
	}
	payload := createCratePayload(t, meta)

	result, err := h.Parse(ctx, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["name"] != "serde" {
		t.Errorf("name = %v, want serde", result["name"])
	}

	if result["vers"] != "1.0.195" {
		t.Errorf("vers = %v, want 1.0.195", result["vers"])
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package raw

import (
	"context"
	"strings"
	"testing"
)

func TestRawHandler_Meta(t *testing.T) {
	h := NewRawHandler()
	if h.Name() != "raw" {
		t.Errorf("Name() = %q, want raw", h.Name())
	}

	if h.GetFileExtension() != "" {
		t.Errorf("GetFileExtension() = %q, want empty", h.GetFileExtension())
	}

	if h.GetContentType() != "application/octet-stream" {
		t.Errorf("GetContentType() = %q, want application/octet-stream", h.GetContentType())
	}
}

func TestRawHandler_Validate(t *testing.T) {
	h := NewRawHandler()
	ctx := context.Background()

	// Any content is valid
	for _, input := range []string{"", "binary data", "some text"} {
		if err := h.Validate(ctx, strings.NewReader(input)); err != nil {
			t.Errorf("Validate(%q) unexpected error: %v", input, err)
		}
	}
}

func TestRawHandler_Parse(t *testing.T) {
	h := NewRawHandler()
	ctx := context.Background()

	result, err := h.Parse(ctx, strings.NewReader("anything"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Parse() = %v, want empty map", result)
	}
}

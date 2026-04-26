// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"context"
	"strings"
	"testing"
)

func TestNPMHandler_Meta(t *testing.T) {
	h := NewNPMHandler()
	if h.Name() != "npm" {
		t.Errorf("Name() = %q, want npm", h.Name())
	}

	if h.GetFileExtension() != ".tgz" {
		t.Errorf("GetFileExtension() = %q, want .tgz", h.GetFileExtension())
	}
}

func TestNPMHandler_Validate_Invalid(t *testing.T) {
	h := NewNPMHandler()

	ctx := context.Background()
	if err := h.Validate(ctx, strings.NewReader("not a tarball")); err == nil {
		t.Error("expected error for invalid npm tarball")
	}
}

func TestNPMHandler_Parse_Invalid(t *testing.T) {
	h := NewNPMHandler()

	ctx := context.Background()
	if _, err := h.Parse(ctx, strings.NewReader("garbage")); err == nil {
		t.Error("expected error for invalid npm tarball")
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package pypi

import (
	"context"
	"strings"
	"testing"
)

func TestPyPIHandler_Meta(t *testing.T) {
	h := NewPyPIHandler()
	if h.Name() != "pypi" {
		t.Errorf("Name() = %q, want pypi", h.Name())
	}

	if h.GetFileExtension() != ".whl" {
		t.Errorf("GetFileExtension() = %q, want .whl", h.GetFileExtension())
	}
}

func TestPyPIHandler_Validate_Invalid(t *testing.T) {
	h := NewPyPIHandler()
	ctx := context.Background()
	// Not a valid wheel or sdist
	if err := h.Validate(ctx, strings.NewReader("not a python package")); err == nil {
		t.Error("expected error for invalid pypi package")
	}
}

func TestPyPIHandler_Parse_Invalid(t *testing.T) {
	h := NewPyPIHandler()

	ctx := context.Background()
	if _, err := h.Parse(ctx, strings.NewReader("garbage")); err == nil {
		t.Error("expected error for invalid pypi package")
	}
}

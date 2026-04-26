// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package conan

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/openartifact/openartifact/internal/format"
)

type ConanHandler struct{}

func NewConanHandler() *ConanHandler {
	return &ConanHandler{}
}

func (h *ConanHandler) Name() string {
	return "conan"
}

func (h *ConanHandler) GetContentType() string {
	return "application/octet-stream"
}

func (h *ConanHandler) GetFileExtension() string {
	return ".tar.gz"
}

func (h *ConanHandler) Validate(ctx context.Context, r io.Reader) error {
	// Basic validation: check if it's a valid tar.gz archive
	buf := make([]byte, 2)

	_, err := r.Read(buf)
	if err != nil {
		return fmt.Errorf("invalid conan package: %w", err)
	}
	// Check for gzip magic bytes (1f 8b)
	if buf[0] != 0x1f || buf[1] != 0x8b {
		return errors.New("invalid conan package: not a valid tar.gz archive") //nolint:err113 // static error
	}

	return nil
}

func (h *ConanHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	// For minimal implementation, return basic metadata
	return map[string]any{
		"format": "conan",
	}, nil
}

var _ format.FormatHandler = (*ConanHandler)(nil)

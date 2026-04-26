// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package conda

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/openartifact/openartifact/internal/format"
)

type CondaHandler struct{}

func NewCondaHandler() *CondaHandler {
	return &CondaHandler{}
}

func (h *CondaHandler) Name() string {
	return "conda"
}

func (h *CondaHandler) GetContentType() string {
	return "application/octet-stream"
}

func (h *CondaHandler) GetFileExtension() string {
	return ".tar.bz2"
}

func (h *CondaHandler) Validate(ctx context.Context, r io.Reader) error {
	// Basic validation: check if it's a valid tar.bz2 or .conda archive
	// For minimal implementation, we just check the magic bytes
	buf := make([]byte, 2)

	_, err := r.Read(buf)
	if err != nil {
		return fmt.Errorf("invalid conda package: %w", err)
	}
	// Check for bzip2 magic bytes (BZ)
	if buf[0] != 0x42 || buf[1] != 0x5a {
		return errors.New("invalid conda package: not a valid tar.bz2 archive") //nolint:err113 // static error
	}

	return nil
}

func (h *CondaHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	// For minimal implementation, return basic metadata
	return map[string]any{
		"format": "conda",
	}, nil
}

var _ format.FormatHandler = (*CondaHandler)(nil)

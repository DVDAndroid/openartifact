// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package ivy

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/openartifact/openartifact/internal/format"
)

type IvyHandler struct{}

func NewIvyHandler() *IvyHandler {
	return &IvyHandler{}
}

func (h *IvyHandler) Name() string {
	return "ivy"
}

func (h *IvyHandler) GetContentType() string {
	return "application/octet-stream"
}

func (h *IvyHandler) GetFileExtension() string {
	return ".jar"
}

func (h *IvyHandler) Validate(ctx context.Context, r io.Reader) error {
	// Basic validation: check if it's a valid JAR (ZIP) archive
	buf := make([]byte, 4)

	_, err := r.Read(buf)
	if err != nil {
		return fmt.Errorf("invalid ivy artifact: %w", err)
	}
	// Check for ZIP magic bytes (50 4b 03 04)
	if buf[0] != 0x50 || buf[1] != 0x4b || buf[2] != 0x03 || buf[3] != 0x04 {
		return errors.New("invalid ivy artifact: not a valid JAR archive") //nolint:err113 // static error
	}

	return nil
}

func (h *IvyHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	return map[string]any{
		"format": "ivy",
	}, nil
}

var _ format.FormatHandler = (*IvyHandler)(nil)

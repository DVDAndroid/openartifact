// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package raw

import (
	"context"
	"io"

	"github.com/openartifact/openartifact/internal/format"
)

type RawHandler struct{}

func NewRawHandler() *RawHandler {
	return &RawHandler{}
}

func (h *RawHandler) Name() string {
	return "raw"
}

func (h *RawHandler) GetContentType() string {
	return "application/octet-stream"
}

func (h *RawHandler) GetFileExtension() string {
	return ""
}

func (h *RawHandler) Validate(ctx context.Context, r io.Reader) error {
	// Any file is valid for raw format
	return nil
}

func (h *RawHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	// Return empty map for raw format
	return make(map[string]any), nil
}

var _ format.FormatHandler = (*RawHandler)(nil)

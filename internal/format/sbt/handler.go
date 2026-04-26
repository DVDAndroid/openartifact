// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package sbt

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/openartifact/openartifact/internal/format"
)

// Sentinel errors for SBT handler.
var (
	ErrInvalidSBTArtifact = errors.New("invalid sbt artifact: not a valid JAR archive")
)

type SBTHandler struct{}

func NewSBTHandler() *SBTHandler {
	return &SBTHandler{}
}

func (h *SBTHandler) Name() string {
	return "sbt"
}

func (h *SBTHandler) GetContentType() string {
	return "application/octet-stream"
}

func (h *SBTHandler) GetFileExtension() string {
	return ".jar"
}

func (h *SBTHandler) Validate(ctx context.Context, r io.Reader) error {
	buf := make([]byte, 4)

	_, err := r.Read(buf)
	if err != nil {
		return fmt.Errorf("invalid sbt artifact: %w", err)
	}

	if buf[0] != 0x50 || buf[1] != 0x4b || buf[2] != 0x03 || buf[3] != 0x04 {
		return ErrInvalidSBTArtifact
	}

	return nil
}

func (h *SBTHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	return map[string]any{
		"format": "sbt",
	}, nil
}

var _ format.FormatHandler = (*SBTHandler)(nil)

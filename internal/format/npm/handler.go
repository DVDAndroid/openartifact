// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"context"
	"fmt"
	"io"

	"github.com/openartifact/openartifact/internal/format"
	pkgnpm "github.com/openartifact/openartifact/pkg/npm"
)

type NPMHandler struct{}

func NewNPMHandler() *NPMHandler {
	return &NPMHandler{}
}

func (h *NPMHandler) Name() string {
	return "npm"
}

func (h *NPMHandler) GetContentType() string {
	return "application/octet-stream"
}

func (h *NPMHandler) GetFileExtension() string {
	return ".tgz"
}

func (h *NPMHandler) Validate(ctx context.Context, r io.Reader) error {
	if err := validateTarball(r); err != nil {
		return fmt.Errorf("invalid npm tarball: %w", err)
	}

	return nil
}

func (h *NPMHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	manifest, _, _, err := pkgnpm.ParseTarball(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse npm tarball: %w", err)
	}

	return manifest, nil
}

// validateTarball checks if the reader contains a valid npm tarball.
func validateTarball(r io.Reader) error {
	manifest, integrity, sha512, err := pkgnpm.ParseTarball(r)
	_ = manifest
	_ = integrity
	_ = sha512

	return err
}

var _ format.FormatHandler = (*NPMHandler)(nil)

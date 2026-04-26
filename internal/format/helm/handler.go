// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package helm

import (
	"context"
	"fmt"
	"io"

	"github.com/openartifact/openartifact/internal/format"
	pkghelm "github.com/openartifact/openartifact/pkg/helm"
)

type HelmHandler struct{}

func NewHelmHandler() *HelmHandler {
	return &HelmHandler{}
}

func (h *HelmHandler) Name() string {
	return "helm"
}

func (h *HelmHandler) GetContentType() string {
	return "application/gzip"
}

func (h *HelmHandler) GetFileExtension() string {
	return ".tgz"
}

func (h *HelmHandler) Validate(ctx context.Context, r io.Reader) error {
	_, err := pkghelm.ParseChart(r)
	if err != nil {
		return fmt.Errorf("invalid helm chart: %w", err)
	}

	return nil
}

func (h *HelmHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	metadata, err := pkghelm.ParseChart(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse helm chart: %w", err)
	}

	return metadata, nil
}

var _ format.FormatHandler = (*HelmHandler)(nil)

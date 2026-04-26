// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rubygems

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/openartifact/openartifact/internal/format"
	"github.com/openartifact/openartifact/pkg/rubymarshal"
)

type RubyGemsHandler struct{}

func NewRubyGemsHandler() *RubyGemsHandler {
	return &RubyGemsHandler{}
}

func (h *RubyGemsHandler) Name() string {
	return "rubygems"
}

func (h *RubyGemsHandler) GetContentType() string {
	return "application/octet-stream"
}

func (h *RubyGemsHandler) GetFileExtension() string {
	return ".gem"
}

// Validate checks if the artifact is a valid .gem file (tar.gz format).
func (h *RubyGemsHandler) Validate(ctx context.Context, r io.Reader) error {
	_, err := h.parseGemMetadata(r)
	if err != nil {
		return fmt.Errorf("invalid gem file: %w", err)
	}

	return nil
}

// Parse extracts metadata from the .gem file.
func (h *RubyGemsHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	metadata, err := h.parseGemMetadata(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gem file: %w", err)
	}

	return metadata, nil
}

// parseGemMetadata extracts metadata from a .gem file.
//
// .gem files are tar archives containing metadata.gz and data.tar.gz. The
// metadata.gz payload is a Ruby Marshal 4.8 dump of the Gem::Specification
// for gems built the way this registry's own upload/legacy-index endpoints
// expect (see pkg/rubymarshal). Real modern `gem build`/`gem push` instead
// write metadata.gz as YAML — parsing that is out of scope here (would need
// a YAML library; this repo's format handlers are stdlib-only) — so JSON is
// kept as a secondary fallback for producers that emit it directly, and a
// raw_metadata passthrough is the last resort.
func (h *RubyGemsHandler) parseGemMetadata(r io.Reader) (map[string]any, error) {
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for metadata.gz file
		if header.Name == "metadata.gz" {
			gzr, err := gzip.NewReader(tr)
			if err != nil {
				return nil, fmt.Errorf("failed to read metadata.gz: %w", err)
			}
			defer func() { _ = gzr.Close() }()

			data, err := io.ReadAll(gzr)
			if err != nil {
				return nil, fmt.Errorf("failed to read metadata content: %w", err)
			}

			return parseGemspecMetadata(data), nil
		}
	}

	// If no metadata.gz found, return minimal metadata
	return map[string]any{
		"name": "unknown",
	}, nil
}

// parseGemspecMetadata decodes a metadata.gz payload, preferring Marshal 4.8
// (a real Gem::Specification dump), falling back to JSON, and finally to a
// raw passthrough. See parseGemMetadata's doc comment for scope.
func parseGemspecMetadata(data []byte) map[string]any {
	if spec, err := rubymarshal.DecodeSpecification(data); err == nil {
		return gemspecToMetadata(spec)
	}

	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err == nil {
		return metadata
	}

	return map[string]any{"raw_metadata": string(data)}
}

func gemspecToMetadata(spec *rubymarshal.Specification) map[string]any {
	deps := make([]map[string]any, len(spec.Dependencies))
	for i, d := range spec.Dependencies {
		deps[i] = map[string]any{
			"name":        d.Name,
			"requirement": d.Requirement,
			"type":        d.Type,
		}
	}

	authors := make([]any, len(spec.Authors))
	for i, a := range spec.Authors {
		authors[i] = a
	}

	licenses := make([]any, len(spec.Licenses))
	for i, l := range spec.Licenses {
		licenses[i] = l
	}

	return map[string]any{
		"name":         spec.Name,
		"version":      spec.Version,
		"platform":     spec.Platform,
		"summary":      spec.Summary,
		"description":  spec.Description,
		"homepage":     spec.Homepage,
		"email":        spec.Email,
		"authors":      authors,
		"licenses":     licenses,
		"dependencies": deps,
	}
}

var _ format.FormatHandler = (*RubyGemsHandler)(nil)

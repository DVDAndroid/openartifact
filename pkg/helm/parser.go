// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package helm

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const maxChartYAMLSize = 1 << 20 // 1 MiB

// Sentinel errors for Helm chart parsing.
var (
	ErrChartYAMLNotFound = errors.New("chart.yaml not found in archive")
	ErrChartYAMLTooLarge = errors.New("Chart.yaml exceeds max size")
)

// readAllLimited reads at most max+1 bytes from r and errors if the
// result exceeds max, without ever allocating more than max+1 bytes.
func readAllLimited(r io.Reader, limit int64, tooLarge error) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: max %d bytes", tooLarge, limit)
	}

	return data, nil
}

// ChartMetadata represents the metadata from Chart.yaml.
type ChartMetadata struct {
	APIVersion  string           `yaml:"apiVersion"`
	Name        string           `yaml:"name"`
	Version     string           `yaml:"version"`
	AppVersion  string           `yaml:"appVersion"`
	Description string           `yaml:"description"`
	Type        string           `yaml:"type"`
	Keywords    []string         `yaml:"keywords"`
	Home        string           `yaml:"home"`
	Sources     []string         `yaml:"sources"`
	Maintainers []map[string]any `yaml:"maintainers"`
	Icon        string           `yaml:"icon"`
}

// ParseChart extracts metadata from a Helm chart tarball.
func ParseChart(r io.Reader) (map[string]any, error) {
	// Decompress gzip
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	// Read tar archive
	tr := tar.NewReader(gr)

	var metadata *ChartMetadata

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		// Look for Chart.yaml in the root of the chart
		if filepath.Base(header.Name) == "Chart.yaml" {
			data, err := readAllLimited(tr, maxChartYAMLSize, ErrChartYAMLTooLarge)
			if err != nil {
				return nil, fmt.Errorf("failed to read Chart.yaml: %w", err)
			}

			metadata = &ChartMetadata{}
			if err := yaml.Unmarshal(data, metadata); err != nil {
				return nil, fmt.Errorf("failed to parse Chart.yaml: %w", err)
			}

			break
		}
	}

	if metadata == nil {
		return nil, ErrChartYAMLNotFound
	}

	// Convert to map[string]interface{}
	result := map[string]any{
		"apiVersion":  metadata.APIVersion,
		"name":        metadata.Name,
		"version":     metadata.Version,
		"appVersion":  metadata.AppVersion,
		"description": metadata.Description,
		"type":        metadata.Type,
		"keywords":    metadata.Keywords,
		"home":        metadata.Home,
		"sources":     metadata.Sources,
		"maintainers": metadata.Maintainers,
		"icon":        metadata.Icon,
	}

	return result, nil
}

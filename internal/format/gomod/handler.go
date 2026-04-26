// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package gomod

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/openartifact/openartifact/internal/format"
	"github.com/openartifact/openartifact/internal/format/base"
)

// maxGoModFileBytes caps the decompressed go.mod entry read from an uploaded
// module zip, preventing a decompression bomb from exhausting memory.
const maxGoModFileBytes = 4 << 20 // 4 MiB

type GoModHandler struct{}

func NewGoModHandler() *GoModHandler {
	return &GoModHandler{}
}

func (h *GoModHandler) Name() string {
	return "gomod"
}

func (h *GoModHandler) GetContentType() string {
	return "application/zip"
}

func (h *GoModHandler) GetFileExtension() string {
	return ".zip"
}

// Validate checks if the artifact is a valid zip containing go.mod.
func (h *GoModHandler) Validate(ctx context.Context, r io.Reader) error {
	// Read into memory to allow multiple reads
	data, err := base.ReadAllLimited(r, maxGoModUploadBytes)
	if err != nil {
		return fmt.Errorf("failed to read zip: %w", err)
	}

	// Try to open as zip
	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid zip file: %w", err)
	}

	// Check for go.mod file
	hasGoMod := false

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "go.mod") {
			hasGoMod = true
			break
		}
	}

	if !hasGoMod {
		return errors.New("zip does not contain go.mod file") //nolint:err113 // static error
	}

	return nil
}

// Parse extracts module path and version from go.mod in the zip.
func (h *GoModHandler) Parse(ctx context.Context, r io.Reader) (map[string]any, error) {
	// Read into memory
	data, err := base.ReadAllLimited(r, maxGoModUploadBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip: %w", err)
	}

	// Open as zip
	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip file: %w", err)
	}

	// Find and read go.mod
	var goModContent string

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "go.mod") {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open go.mod: %w", err)
			}
			defer func() { _ = rc.Close() }()

			content, err := base.ReadAllLimited(rc, maxGoModFileBytes)
			if err != nil {
				return nil, fmt.Errorf("failed to read go.mod: %w", err)
			}

			goModContent = string(content)

			break
		}
	}

	if goModContent == "" {
		return nil, errors.New("go.mod not found in zip") //nolint:err113 // static error
	}

	// Parse module path from go.mod
	modulePath := extractModulePath(goModContent)
	if modulePath == "" {
		return nil, errors.New("failed to extract module path from go.mod") //nolint:err113 // static error
	}

	return map[string]any{
		"module_path": modulePath,
		"go_mod":      goModContent,
	}, nil
}

// extractModulePath extracts the module path from go.mod content.
func extractModulePath(goModContent string) string {
	lines := strings.SplitSeq(goModContent, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return ""
}

var _ format.FormatHandler = (*GoModHandler)(nil)

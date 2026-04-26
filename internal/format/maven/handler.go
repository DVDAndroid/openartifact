// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/openartifact/openartifact/internal/format"
	pkgmaven "github.com/openartifact/openartifact/pkg/maven"
)

type MavenHandler struct{}

func NewMavenHandler() *MavenHandler {
	return &MavenHandler{}
}

func (h *MavenHandler) Name() string {
	return "maven"
}

func (h *MavenHandler) GetContentType() string {
	return "application/octet-stream"
}

func ContentTypeForFile(filename string) string {
	switch filepath.Ext(filename) {
	case ".jar":
		return "application/java-archive"
	case ".pom":
		return "application/xml"
	case ".war":
		return "application/java-archive"
	case ".aar":
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}

func (h *MavenHandler) GetFileExtension() string {
	return ""
}

func (h *MavenHandler) Validate(ctx context.Context, data io.Reader) error {
	return ValidateFile(data, "")
}

func ValidateFile(r io.Reader, filename string) error {
	if filepath.Ext(filename) == ".pom" {
		_, err := pkgmaven.ParsePOM(r)
		if err != nil {
			return fmt.Errorf("invalid pom file: %w", err)
		}

		return nil
	}

	buf := make([]byte, 1)

	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read artifact: %w", err)
	}

	if n == 0 {
		return errors.New("artifact is empty") //nolint:err113 // static error
	}

	return nil
}

func (h *MavenHandler) Parse(ctx context.Context, data io.Reader) (map[string]any, error) {
	return nil, nil //nolint:nilnil // standard Go not-found pattern: nil pointer with nil error
}

func ParseFile(r io.Reader, filename string) (any, error) {
	if filepath.Ext(filename) == ".pom" {
		return pkgmaven.ParsePOM(r)
	}

	return nil, nil //nolint:nilnil // standard Go not-found pattern: nil pointer with nil error
}

var _ format.FormatHandler = (*MavenHandler)(nil)

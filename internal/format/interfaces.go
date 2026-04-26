// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package format

import (
	"context"
	"io"
)

// FormatHandler defines the interface for handling artifact format operations.
type FormatHandler interface {
	// Name returns the format name (e.g., "docker", "maven", "npm")
	Name() string

	// Validate checks if the artifact conforms to the format specification
	Validate(ctx context.Context, data io.Reader) error

	// Parse extracts metadata from the artifact
	Parse(ctx context.Context, data io.Reader) (map[string]any, error)

	// GetContentType returns the MIME type for this format
	GetContentType() string

	// GetFileExtension returns the typical file extension for this format
	GetFileExtension() string
}

// FormatRegistry manages registered format handlers.
type FormatRegistry interface {
	// Register adds a new format handler
	Register(handler FormatHandler) error

	// Get retrieves a handler by format name
	Get(name string) (FormatHandler, error)

	// List returns all registered format names
	List() []string

	// Unregister removes a format handler
	Unregister(name string) error
}

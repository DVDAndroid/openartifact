// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"

	"github.com/go-chi/chi/v5"
)

// Handler defines the interface for API handlers.
type Handler interface {
	// Register registers the handler's routes
	Register(router chi.Router)

	// Name returns the handler name
	Name() string
}

// Server defines the interface for the HTTP server.
type Server interface {
	// Start starts the server
	Start(ctx context.Context) error

	// Stop gracefully stops the server
	Stop(ctx context.Context) error

	// RegisterHandler registers an API handler
	RegisterHandler(handler Handler) error

	// GetRouter returns the underlying router
	GetRouter() chi.Router
}

// Response represents a standard API response.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

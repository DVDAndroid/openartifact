// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package format

import (
	"sync"

	"github.com/openartifact/openartifact/pkg/errors"
)

// DefaultRegistry implements the FormatRegistry interface.
type DefaultRegistry struct {
	handlers map[string]FormatHandler
	mu       sync.RWMutex
}

// NewRegistry creates a new format registry.
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		handlers: make(map[string]FormatHandler),
	}
}

// Register adds a new format handler.
func (r *DefaultRegistry) Register(handler FormatHandler) error {
	if handler == nil {
		return errors.NewError(errors.ErrInvalidInput).AddDetail("reason", "handler cannot be nil")
	}

	name := handler.Name()
	if name == "" {
		return errors.NewError(errors.ErrInvalidInput).AddDetail("reason", "handler name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[name]; exists {
		return errors.NewError(errors.ErrAlreadyExists).AddDetail("format", name)
	}

	r.handlers[name] = handler

	return nil
}

// Get retrieves a handler by format name.
func (r *DefaultRegistry) Get(name string) (FormatHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[name]
	if !exists {
		return nil, errors.NewError(errors.ErrFormatNotFound).AddDetail("format", name)
	}

	return handler, nil
}

// List returns all registered format names.
func (r *DefaultRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}

	return names
}

// Unregister removes a format handler.
func (r *DefaultRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[name]; !exists {
		return errors.NewError(errors.ErrFormatNotFound).AddDetail("format", name)
	}

	delete(r.handlers, name)

	return nil
}

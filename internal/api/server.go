// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/openartifact/openartifact/pkg/logger"
)

var errHandlerNil = errors.New("handler cannot be nil")

// HTTPServer implements the Server interface.
type HTTPServer struct {
	router      chi.Router
	httpServer  *http.Server
	log         *logger.Logger
	handlers    []Handler
	middlewares []func(http.Handler) http.Handler
}

// NewHTTPServer creates a new HTTP server. Defaults favor never truncating
// large artifact transfers: no read/write timeout, with only header reads
// bounded (slowloris mitigation) until SetTimeouts overrides them from config.
func NewHTTPServer(addr string, log *logger.Logger) *HTTPServer {
	r := chi.NewRouter()

	return &HTTPServer{
		router: r,
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadTimeout:       0,
			WriteTimeout:      0,
			ReadHeaderTimeout: 20 * time.Second,
		},
		log:      log,
		handlers: make([]Handler, 0),
	}
}

// Start starts the server.
func (s *HTTPServer) Start(ctx context.Context) error {
	s.log.Infow("Starting HTTP server", "address", s.httpServer.Addr)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Errorw("HTTP server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the server.
func (s *HTTPServer) Stop(ctx context.Context) error {
	s.log.Infow("Stopping HTTP server")
	return s.httpServer.Shutdown(ctx)
}

// RegisterHandler registers an API handler.
func (s *HTTPServer) RegisterHandler(handler Handler) error {
	if handler == nil {
		return errHandlerNil
	}

	handler.Register(s.router)

	s.handlers = append(s.handlers, handler)
	s.log.Infow("Handler registered", "name", handler.Name())

	return nil
}

// GetRouter returns the underlying chi router.
func (s *HTTPServer) GetRouter() chi.Router {
	return s.router
}

// AddMiddleware adds a middleware to the server.
func (s *HTTPServer) AddMiddleware(middleware func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middleware)
	s.httpServer.Handler = applyMiddlewares(s.router, s.middlewares...)
}

// applyMiddlewares applies middlewares to a handler.
func applyMiddlewares(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Apply middlewares in reverse order so they execute in the order they were added
	for _, m := range slices.Backward(middlewares) {
		handler = m(handler)
	}

	return handler
}

// SetTimeouts configures read, write, and read-header timeouts on the HTTP
// server. readTimeout/writeTimeout of 0 mean "no limit" (relevant for large
// artifact transfers); readHeaderTimeout should stay non-zero regardless —
// it bounds only the header-read phase, so it never truncates a body.
func (s *HTTPServer) SetTimeouts(readTimeout, writeTimeout, readHeaderTimeout time.Duration) {
	s.httpServer.ReadTimeout = readTimeout
	s.httpServer.WriteTimeout = writeTimeout
	s.httpServer.ReadHeaderTimeout = readHeaderTimeout
}

// Ensure HTTPServer implements Server interface.
var _ Server = (*HTTPServer)(nil)

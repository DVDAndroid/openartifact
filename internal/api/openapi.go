// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	_ "embed"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed openapi.json
var openapiSpec []byte

type OpenAPIHandler struct{}

func NewOpenAPIHandler() *OpenAPIHandler {
	return &OpenAPIHandler{}
}

func (h *OpenAPIHandler) RegisterRoutes(router chi.Router) {
	router.Get("/api/v1/openapi.json", h.handleSpec)
	router.Get("/api/v1/openapi.yaml", h.handleSpecYAML)
	router.Get("/swagger/", h.handleSwaggerUI)
	router.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
}

func (h *OpenAPIHandler) handleSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(openapiSpec)
}

func (h *OpenAPIHandler) handleSpecYAML(w http.ResponseWriter, r *http.Request) {
	// For simplicity, serve the JSON with yaml content-type
	// JSON is a valid YAML superset
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(openapiSpec)
}

func (h *OpenAPIHandler) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	// Serve inline Swagger UI HTML that loads from CDN
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(swaggerUIHTML))
}

const swaggerUIHTML = `<!DOCTYPE html>
<html>
<head>
  <title>OpenArtifact API</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" >
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"> </script>
<script>
window.onload = function() {
  SwaggerUIBundle({
    url: "/api/v1/openapi.json",
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
    layout: "BaseLayout"
  })
}
</script>
</body>
</html>`

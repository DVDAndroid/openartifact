# SPDX-FileCopyrightText: The OpenArtifact Authors
#
# SPDX-License-Identifier: AGPL-3.0-or-later

# ---- Frontend build stage ---------------------------------------------------
# Builds the Vue 3 SPA so web/dist/ is populated before Go's //go:embed runs.
FROM docker.io/oven/bun:1-alpine AS web-builder

WORKDIR /web

# Install JS deps with cache-friendly layering
COPY web/package.json web/bun.lock* web/bun.lockb* ./
RUN bun install --frozen-lockfile || bun install

# Build the SPA
COPY web/ ./

# Skip vue-tsc type-check (known musl compat issue with TS 6 + project references).
# Vite build catches broken imports; run `vue-tsc --noEmit` locally or in CI.
RUN bunx vite build
# ---- Backend build stage ----------------------------------------------------
FROM docker.io/golang:1.27-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Replace web/dist with the freshly built assets so //go:embed all:dist
# picks up the real SPA, not the .gitkeep placeholder.
COPY --from=web-builder /web/dist ./web/dist

# Build the application
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o openartifact ./cmd/server

# Runtime stage
FROM docker.io/alpine:3.24

WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/openartifact .

# Create artifacts directory
RUN mkdir -p /var/lib/artifacts

# Run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    chown -R appuser:appgroup /var/lib/artifacts /app
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./openartifact"]

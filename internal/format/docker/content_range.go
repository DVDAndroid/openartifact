// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package docker

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrChunkOffsetMismatch indicates a chunked upload PATCH/PUT arrived with a
// Content-Range start that does not match the upload's current offset —
// either a gap (client skipped bytes) or an overlap (client resent bytes
// already accepted). Registries reject both with 416 Range Not Satisfiable.
var ErrChunkOffsetMismatch = errors.New("chunk offset does not match current upload offset")

// ErrInvalidContentRange indicates a missing or malformed Content-Range header.
var ErrInvalidContentRange = errors.New("invalid Content-Range")

// parseContentRange parses a "Content-Range: <start>-<end>" header value, as
// used by Docker/OCI chunked blob upload PATCH/PUT requests. It does not
// accept the HTTP/1.1 "bytes " prefix or "*" total form since those are not
// part of the registry protocol's Content-Range usage.
func parseContentRange(header string) (int64, int64, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, 0, fmt.Errorf("%w: empty header", ErrInvalidContentRange)
	}

	parts := strings.SplitN(header, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w %q: want <start>-<end>", ErrInvalidContentRange, header)
	}

	start, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid Content-Range start %q: %w", header, err)
	}

	end, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid Content-Range end %q: %w", header, err)
	}

	if end < start {
		return 0, 0, fmt.Errorf("%w %q: end before start", ErrInvalidContentRange, header)
	}

	return start, end, nil
}

// validateChunkOffset checks that a chunk beginning at start is contiguous
// with currentOffset — the only byte range a registry can append without
// leaving a gap or silently overwriting already-accepted bytes.
func validateChunkOffset(currentOffset, start int64) error {
	if start != currentOffset {
		return fmt.Errorf("%w: expected start %d, got %d", ErrChunkOffsetMismatch, currentOffset, start)
	}

	return nil
}

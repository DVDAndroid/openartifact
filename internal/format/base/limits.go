// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package base

import (
	"errors"
	"fmt"
	"io"
	"math"
)

// ErrContentTooLarge reports that a reader exceeded its content limit.
var ErrContentTooLarge = errors.New("content too large")

// ErrNegativeContentLimit reports a negative content limit.
var ErrNegativeContentLimit = errors.New("negative content limit")

// ReadAllLimited reads r up to maxBytes and reports when the limit is exceeded.
func ReadAllLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes < 0 {
		return nil, fmt.Errorf("%w: %d", ErrNegativeContentLimit, maxBytes)
	}

	limit := maxBytes
	if limit < math.MaxInt64 {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(r, limit))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: limit is %d bytes", ErrContentTooLarge, maxBytes)
	}
	return data, nil
}

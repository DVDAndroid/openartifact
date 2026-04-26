// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrInvalidAttachment = errors.New("invalid npm attachment")

func DecodeAttachment(att map[string]any) ([]byte, string, error) {
	ct, ok := att["content_type"].(string)
	if !ok {
		return nil, "", fmt.Errorf("%w: content_type missing or not a string", ErrInvalidAttachment)
	}

	if ct != "application/octet-stream" {
		return nil, "", fmt.Errorf("%w: content_type must be application/octet-stream, got %q", ErrInvalidAttachment, ct)
	}

	dataStr, ok := att["data"].(string)
	if !ok {
		return nil, "", fmt.Errorf("%w: data missing or not a string", ErrInvalidAttachment)
	}

	decoded, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, "", fmt.Errorf("%w: base64 decode: %w", ErrInvalidAttachment, err)
	}

	var declaredLen int64

	switch v := att["length"].(type) {
	case json.Number:
		declaredLen, err = v.Int64()
		if err != nil {
			return nil, "", fmt.Errorf("%w: length parse: %w", ErrInvalidAttachment, err)
		}
	case float64:
		declaredLen = int64(v)
	default:
		return nil, "", fmt.Errorf("%w: length missing or invalid type", ErrInvalidAttachment)
	}

	if int64(len(decoded)) != declaredLen {
		return nil, "", fmt.Errorf("%w: length mismatch: declared %d, actual %d", ErrInvalidAttachment, declaredLen, len(decoded))
	}

	return decoded, ct, nil
}

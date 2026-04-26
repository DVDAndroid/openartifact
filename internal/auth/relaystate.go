// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

var ErrInvalidRelayState = errors.New("invalid relay state")

func SignRelayState(redirect, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(redirect))
	sig := hex.EncodeToString(mac.Sum(nil))

	return base64.RawURLEncoding.EncodeToString([]byte(redirect + "|" + sig))
}

func VerifyRelayState(encoded, key string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrInvalidRelayState
	}

	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return "", ErrInvalidRelayState
	}

	redirect, gotSig := parts[0], parts[1]
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(redirect))

	expSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(gotSig), []byte(expSig)) {
		return "", ErrInvalidRelayState
	}

	return redirect, nil
}

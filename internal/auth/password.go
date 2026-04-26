// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 1
	argonMemory  = 65536
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// argonSemaphore caps concurrent Argon2id operations to prevent OOM under load.
//
//nolint:gochecknoglobals // rate-limits concurrent Argon2id hashing
var argonSemaphore = make(chan struct{}, runtime.NumCPU())

// HashPassword hashes a plaintext password using Argon2id and returns an encoded string.
// Format: $argon2id$v=19$m=65536,t=1,p=4$<base64-no-padding-salt>$<base64-no-padding-hash>.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	argonSemaphore <- struct{}{}

	hash := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	<-argonSemaphore

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads, b64Salt, b64Hash), nil
}

// VerifyPassword checks a plaintext password against an encoded Argon2id hash.
// Returns (true, nil) on match, (false, nil) on mismatch, (false, err) on malformed input.
func VerifyPassword(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=65536,t=1,p=4", "<salt>", "<hash>"]
	if len(parts) != 6 {
		return false, fmt.Errorf("invalid encoded hash: expected 6 segments, got %d", len(parts)) //nolint:err113 // dynamic error with runtime context
	}

	if parts[1] != "argon2id" {
		return false, fmt.Errorf("invalid algorithm: %s", parts[1]) //nolint:err113 // dynamic error with runtime context
	}

	var (
		memory, time uint32
		threads      uint8
	)
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, fmt.Errorf("parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("decode hash: %w", err)
	}

	argonSemaphore <- struct{}{}

	derivedHash := argon2.IDKey([]byte(plain), salt, time, memory, threads, uint32(len(expectedHash))) //nolint:gosec // argon2 requires uint32 for key length

	<-argonSemaphore

	if subtle.ConstantTimeCompare(derivedHash, expectedHash) != 1 {
		return false, nil
	}

	return true, nil
}

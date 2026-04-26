// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openartifact/openartifact/internal/storage"
)

func TestFilesystemStore_PresignURL_ReturnsErrPresignNotSupported(t *testing.T) {
	dir := t.TempDir()

	store, err := storage.NewFilesystemStore(dir)
	if err != nil {
		t.Fatalf("NewFilesystemStore: %v", err)
	}

	url, err := store.PresignURL(context.Background(), "abc123", 5*time.Minute)
	if url != "" {
		t.Errorf("expected empty URL, got %q", url)
	}

	if !errors.Is(err, storage.ErrPresignNotSupported) {
		t.Errorf("expected ErrPresignNotSupported, got %v", err)
	}
}

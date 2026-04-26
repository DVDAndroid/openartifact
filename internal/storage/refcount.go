// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"fmt"

	"github.com/openartifact/openartifact/internal/database"
)

// SafeDeleteBlob deletes the blob identified by sha256 from blobStore only if
// no package-table row still references it (db.BlobReferenceCount == 0).
//
// Callers MUST delete their own DB row referencing the blob BEFORE calling
// this function, otherwise BlobReferenceCount will still see that row and
// the blob will never be removed from storage.
func SafeDeleteBlob(ctx context.Context, db database.Database, blobStore BlobStore, sha256 string) error {
	count, err := db.BlobReferenceCount(ctx, sha256)
	if err != nil {
		return fmt.Errorf("failed to check blob reference count: %w", err)
	}

	if count > 0 {
		return nil
	}

	if err := blobStore.Delete(ctx, sha256); err != nil {
		return fmt.Errorf("failed to delete blob from storage: %w", err)
	}

	return nil
}

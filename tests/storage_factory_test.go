// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"testing"

	"github.com/openartifact/openartifact/internal/storage"
)

func TestNewBlobStore_FilesystemEmpty(t *testing.T) {
	dir := t.TempDir()

	store, err := storage.NewBlobStore(storage.StorageConfig{Type: "", Path: dir})
	if err != nil {
		t.Fatalf("NewBlobStore(filesystem): %v", err)
	}

	if _, ok := store.(*storage.FilesystemStore); !ok {
		t.Errorf("expected *FilesystemStore, got %T", store)
	}
}

func TestNewBlobStore_FilesystemExplicit(t *testing.T) {
	dir := t.TempDir()

	store, err := storage.NewBlobStore(storage.StorageConfig{Type: "filesystem", Path: dir})
	if err != nil {
		t.Fatalf("NewBlobStore(filesystem): %v", err)
	}

	if _, ok := store.(*storage.FilesystemStore); !ok {
		t.Errorf("expected *FilesystemStore, got %T", store)
	}
}

func TestNewBlobStore_S3(t *testing.T) {
	cfg := storage.StorageConfig{
		Type:           "s3",
		Endpoint:       "http://localhost:9000",
		Bucket:         "test",
		Region:         "us-east-1",
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		ForcePathStyle: true,
	}

	store, err := storage.NewBlobStore(cfg)
	if err != nil {
		t.Fatalf("NewBlobStore(s3): %v", err)
	}

	if _, ok := store.(*storage.S3BlobStore); !ok {
		t.Errorf("expected *S3BlobStore, got %T", store)
	}
}

func TestNewBlobStore_UnknownType(t *testing.T) {
	_, err := storage.NewBlobStore(storage.StorageConfig{Type: "gcs"})
	if err == nil {
		t.Fatal("expected error for unknown storage type")
	}
}

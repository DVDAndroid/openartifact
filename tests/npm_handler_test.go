// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"testing"

	fmtnpm "github.com/openartifact/openartifact/internal/format/npm"
)

func buildMinimalTarball(t *testing.T, manifest map[string]any) []byte {
	t.Helper()

	jsonBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: "package/package.json",
		Mode: 0o644,
		Size: int64(len(jsonBytes)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar write header: %v", err)
	}

	if _, err := tw.Write(jsonBytes); err != nil {
		t.Fatalf("tar write: %v", err)
	}

	_ = tw.Close()
	_ = gw.Close()

	return buf.Bytes()
}

func TestNPMHandler_Name(t *testing.T) {
	h := fmtnpm.NewNPMHandler()
	if h.Name() != "npm" {
		t.Errorf("Name() = %q, want %q", h.Name(), "npm")
	}
}

func TestNPMHandler_GetContentType(t *testing.T) {
	h := fmtnpm.NewNPMHandler()
	if h.GetContentType() != "application/octet-stream" {
		t.Errorf("GetContentType() = %q, want %q", h.GetContentType(), "application/octet-stream")
	}
}

func TestNPMHandler_GetFileExtension(t *testing.T) {
	h := fmtnpm.NewNPMHandler()
	if h.GetFileExtension() != ".tgz" {
		t.Errorf("GetFileExtension() = %q, want %q", h.GetFileExtension(), ".tgz")
	}
}

func TestNPMHandler_Validate_Success(t *testing.T) {
	data := buildMinimalTarball(t, map[string]any{"name": "mypackage", "version": "1.0.0"})

	h := fmtnpm.NewNPMHandler()
	if err := h.Validate(context.Background(), bytes.NewReader(data)); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestNPMHandler_Validate_Failure(t *testing.T) {
	h := fmtnpm.NewNPMHandler()
	if err := h.Validate(context.Background(), bytes.NewReader([]byte("not a tarball"))); err == nil {
		t.Error("Validate() expected error for invalid tarball")
	}
}

func TestNPMHandler_Parse(t *testing.T) {
	data := buildMinimalTarball(t, map[string]any{"name": "mypackage", "version": "1.0.0"})
	h := fmtnpm.NewNPMHandler()

	m, err := h.Parse(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse() unexpected error: %v", err)
	}

	if m["name"] != "mypackage" {
		t.Errorf("Parse() name = %v, want %q", m["name"], "mypackage")
	}
}

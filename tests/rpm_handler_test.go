// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"bytes"
	"context"
	"testing"

	formatrpm "github.com/openartifact/openartifact/internal/format/rpm"
)

func TestRPMHandler_Name(t *testing.T) {
	h := formatrpm.NewRPMHandler()
	if h.Name() != "rpm" {
		t.Errorf("Name() = %q, want %q", h.Name(), "rpm")
	}
}

func TestRPMHandler_GetContentType(t *testing.T) {
	h := formatrpm.NewRPMHandler()
	if h.GetContentType() != "application/x-rpm" {
		t.Errorf("GetContentType() = %q, want %q", h.GetContentType(), "application/x-rpm")
	}
}

func TestRPMHandler_GetFileExtension(t *testing.T) {
	h := formatrpm.NewRPMHandler()
	if h.GetFileExtension() != ".rpm" {
		t.Errorf("GetFileExtension() = %q, want %q", h.GetFileExtension(), ".rpm")
	}
}

func TestRPMHandler_Validate_Valid(t *testing.T) {
	h := formatrpm.NewRPMHandler()
	rpmData := buildMinimalRPM()

	err := h.Validate(context.Background(), bytes.NewReader(rpmData))
	if err != nil {
		t.Errorf("Validate(valid RPM) error: %v", err)
	}
}

func TestRPMHandler_Validate_Invalid(t *testing.T) {
	h := formatrpm.NewRPMHandler()

	err := h.Validate(context.Background(), bytes.NewReader([]byte("not an rpm")))
	if err == nil {
		t.Error("Validate(invalid data) expected error, got nil")
	}
}

func TestRPMHandler_Parse_Valid(t *testing.T) {
	h := formatrpm.NewRPMHandler()
	rpmData := buildMinimalRPM()

	metadata, err := h.Parse(context.Background(), bytes.NewReader(rpmData))
	if err != nil {
		t.Fatalf("Parse(valid RPM) error: %v", err)
	}

	if metadata["name"] != "rpmtest" {
		t.Errorf("metadata[name] = %v, want rpmtest", metadata["name"])
	}

	if metadata["format"] != "rpm" {
		t.Errorf("metadata[format] = %v, want rpm", metadata["format"])
	}

	if metadata["file_type"] != "rpm" {
		t.Errorf("metadata[file_type] = %v, want rpm", metadata["file_type"])
	}
}

func TestRPMHandler_Parse_Invalid(t *testing.T) {
	h := formatrpm.NewRPMHandler()

	_, err := h.Parse(context.Background(), bytes.NewReader([]byte("garbage")))
	if err == nil {
		t.Error("Parse(invalid data) expected error, got nil")
	}
}

func TestRepositoryHandler_ValidatePackage_Valid(t *testing.T) {
	rh := formatrpm.NewRepositoryHandler()
	rpmData := buildMinimalRPM()

	pkg, err := rh.ValidatePackage(context.Background(), bytes.NewReader(rpmData))
	if err != nil {
		t.Fatalf("ValidatePackage(valid) error: %v", err)
	}

	if pkg.Name != "rpmtest" {
		t.Errorf("Name = %q, want %q", pkg.Name, "rpmtest")
	}
}

func TestRepositoryHandler_ValidatePackage_Invalid(t *testing.T) {
	rh := formatrpm.NewRepositoryHandler()

	_, err := rh.ValidatePackage(context.Background(), bytes.NewReader([]byte("not an rpm")))
	if err == nil {
		t.Error("ValidatePackage(invalid) expected error, got nil")
	}
}

func TestRepositoryHandler_GetPackageInfo(t *testing.T) {
	rh := formatrpm.NewRepositoryHandler()
	rpmData := buildMinimalRPM()
	pkg, _ := rh.ValidatePackage(context.Background(), bytes.NewReader(rpmData))

	info := rh.GetPackageInfo(pkg)
	if info["name"] != "rpmtest" {
		t.Errorf("info[name] = %v, want rpmtest", info["name"])
	}

	nilInfo := rh.GetPackageInfo(nil)
	if len(nilInfo) != 0 {
		t.Errorf("GetPackageInfo(nil) = %v, want empty map", nilInfo)
	}
}

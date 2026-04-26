// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	fmtmaven "github.com/openartifact/openartifact/internal/format/maven"
	pkgmaven "github.com/openartifact/openartifact/pkg/maven"
)

func TestMavenHandler_Name(t *testing.T) {
	h := fmtmaven.NewMavenHandler()
	if h.Name() != "maven" {
		t.Errorf("Name() = %q, want %q", h.Name(), "maven")
	}
}

func TestMavenHandler_GetFileExtension(t *testing.T) {
	h := fmtmaven.NewMavenHandler()
	if h.GetFileExtension() != "" {
		t.Errorf("GetFileExtension() = %q, want %q", h.GetFileExtension(), "")
	}
}

func TestMavenHandler_ContentTypeForFile(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"myapp-1.0.0.jar", "application/java-archive"},
		{"myapp-1.0.0.pom", "application/xml"},
		{"myapp-1.0.0.zip", "application/octet-stream"},
	}
	for _, tc := range tests {
		got := fmtmaven.ContentTypeForFile(tc.filename)
		if got != tc.want {
			t.Errorf("ContentTypeForFile(%q) = %q, want %q", tc.filename, got, tc.want)
		}
	}
}

func TestMavenHandler_ValidateFile_POM_Valid(t *testing.T) {
	validPOM := `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <groupId>com.example</groupId>
  <artifactId>myapp</artifactId>
  <version>1.0.0</version>
</project>`

	err := fmtmaven.ValidateFile(strings.NewReader(validPOM), "myapp-1.0.0.pom")
	if err != nil {
		t.Errorf("ValidateFile(valid POM) unexpected error: %v", err)
	}
}

func TestMavenHandler_ValidateFile_POM_Invalid(t *testing.T) {
	err := fmtmaven.ValidateFile(strings.NewReader("not xml garbage {{{{"), "myapp-1.0.0.pom")
	if err == nil {
		t.Error("ValidateFile(invalid POM) expected error, got nil")
	}
}

func TestMavenHandler_ValidateFile_JAR(t *testing.T) {
	err := fmtmaven.ValidateFile(bytes.NewReader([]byte{0x50, 0x4b, 0x03, 0x04}), "myapp-1.0.0.jar")
	if err != nil {
		t.Errorf("ValidateFile(non-empty JAR) unexpected error: %v", err)
	}
}

func TestMavenHandler_ParseFile_POM(t *testing.T) {
	pomXML := `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <groupId>com.example</groupId>
  <artifactId>myapp</artifactId>
  <version>1.2.3</version>
  <packaging>jar</packaging>
</project>`

	result, err := fmtmaven.ParseFile(strings.NewReader(pomXML), "myapp-1.2.3.pom")
	if err != nil {
		t.Fatalf("ParseFile(valid POM) unexpected error: %v", err)
	}

	pom, ok := result.(*pkgmaven.POM)
	if !ok {
		t.Fatalf("ParseFile returned %T, want *pkgmaven.POM", result)
	}

	if pom.GroupID != "com.example" {
		t.Errorf("POM.GroupID = %q, want %q", pom.GroupID, "com.example")
	}

	if pom.ArtifactID != "myapp" {
		t.Errorf("POM.ArtifactID = %q, want %q", pom.ArtifactID, "myapp")
	}

	if pom.Version != "1.2.3" {
		t.Errorf("POM.Version = %q, want %q", pom.Version, "1.2.3")
	}
}

func TestMavenHandler_Validate_Context(t *testing.T) {
	h := fmtmaven.NewMavenHandler()

	err := h.Validate(context.Background(), bytes.NewReader([]byte{0x01}))
	if err != nil {
		t.Errorf("Validate(non-empty bytes) unexpected error: %v", err)
	}
}

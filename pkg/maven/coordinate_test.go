// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"errors"
	"testing"
)

func TestParseCoordinate_Valid(t *testing.T) {
	c, err := ParseCoordinate("com.example", "my-app", "1.0.0", "sources", "jar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.GroupID != "com.example" || c.ArtifactID != "my-app" || c.Version != "1.0.0" ||
		c.Classifier != "sources" || c.Extension != "jar" {
		t.Fatalf("unexpected coordinate: %+v", c)
	}

	c2, err := ParseCoordinate("org.apache", "commons-lang3", "3.12.0", "", "pom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c2.Classifier != "" || c2.Extension != "pom" {
		t.Fatalf("unexpected coordinate: %+v", c2)
	}
}

func TestParseCoordinate_InvalidGroupID(t *testing.T) {
	_, err := ParseCoordinate("com..foo", "app", "1.0", "", "jar")
	if err == nil {
		t.Fatal("expected error for groupID with ..")
	}

	if !errors.Is(err, ErrInvalidCoordinate) {
		t.Fatalf("expected ErrInvalidCoordinate, got %v", err)
	}
}

func TestParseCoordinate_EmptyVersion(t *testing.T) {
	_, err := ParseCoordinate("com.example", "app", "", "", "jar")
	if err == nil {
		t.Fatal("expected error for empty version")
	}

	if !errors.Is(err, ErrInvalidCoordinate) {
		t.Fatalf("expected ErrInvalidCoordinate, got %v", err)
	}
}

func TestRepositoryPath_RoundTrip(t *testing.T) {
	c := Coordinate{
		GroupID:    "com.example",
		ArtifactID: "my-app",
		Version:    "1.0.0",
		Classifier: "",
		Extension:  "jar",
	}
	path := RepositoryPath(c)

	groupPath, artifactID, version, filename, err := ParseRepositoryPath(path)
	if err != nil {
		t.Fatalf("ParseRepositoryPath error: %v", err)
	}

	if groupPath != "com/example" {
		t.Errorf("groupPath = %q, want %q", groupPath, "com/example")
	}

	if artifactID != "my-app" {
		t.Errorf("artifactID = %q, want %q", artifactID, "my-app")
	}

	if version != "1.0.0" {
		t.Errorf("version = %q, want %q", version, "1.0.0")
	}

	if filename != "my-app-1.0.0.jar" {
		t.Errorf("filename = %q, want %q", filename, "my-app-1.0.0.jar")
	}
}

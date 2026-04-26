// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"context"
	"strings"
	"testing"
)

func TestMavenHandler_Meta(t *testing.T) {
	h := NewMavenHandler()
	if h.Name() != "maven" {
		t.Errorf("Name() = %q, want maven", h.Name())
	}
}

func TestContentTypeForFile(t *testing.T) {
	cases := []struct{ filename, want string }{
		{"foo.jar", "application/java-archive"},
		{"foo.pom", "application/xml"},
		{"foo.war", "application/java-archive"},
		{"foo.aar", "application/octet-stream"},
		{"foo.unknown", "application/octet-stream"},
	}
	for _, c := range cases {
		got := ContentTypeForFile(c.filename)
		if got != c.want {
			t.Errorf("ContentTypeForFile(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}

func TestMavenHandler_Validate_Invalid(t *testing.T) {
	h := NewMavenHandler()
	ctx := context.Background()
	// Empty reader → error
	if err := h.Validate(ctx, strings.NewReader("")); err == nil {
		t.Error("expected error for empty artifact")
	}
}

func TestMavenHandler_Parse(t *testing.T) {
	h := NewMavenHandler()
	ctx := context.Background()
	// Parse returns nil,nil for non-pom files
	result, err := h.Parse(ctx, strings.NewReader("anything"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_ = result
}

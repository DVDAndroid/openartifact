// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"strings"
	"testing"

	"github.com/openartifact/openartifact/internal/database/models"
)

func TestPickIcon(t *testing.T) {
	icons := []models.AndroidIcon{
		{ID: "small", Role: "app", Width: 48, BlobID: "b", Mime: "image/png"},
		{ID: "mid", Role: "app", Width: 96, BlobID: "b", Mime: "image/webp"},
		{ID: "big", Role: "app", Width: 192, BlobID: "b", Mime: "image/png"},
		{ID: "vector", Role: "app", Width: 512, BlobID: "b", Mime: "application/vnd.android.vector+xml"},
		{ID: "nodata", Role: "app", Width: 1024, Mime: "image/png"},
		{ID: "fg", Role: "foreground", Width: 108, BlobID: "b", Mime: "image/png"},
	}

	for _, tt := range []struct {
		role string
		size int
		want string
	}{
		{"app", 40, "small"}, {"app", 49, "mid"}, {"app", 96, "mid"}, {"app", 150, "big"},
		{"app", 5000, "big"}, {"app", 0, "big"}, {"foreground", 0, "fg"}, {"monochrome", 0, ""},
	} {
		got := ""
		if ic := pickIcon(icons, tt.role, tt.size); ic != nil {
			got = ic.ID
		}

		if got != tt.want {
			t.Errorf("pickIcon(%s, %d) = %q, want %q", tt.role, tt.size, got, tt.want)
		}
	}
}

func TestSafeFileName(t *testing.T) {
	for in, want := range map[string]string{
		"../../etc/passwd":       "passwd",
		`..\..\boot.ini`:         "boot.ini",
		"mapping v2.txt":         "mapping_v2.txt",
		".hidden":                "hidden",
		"":                       "file",
		"..":                     "file",
		"<script>x.html":         "_script_x.html",
		strings.Repeat("a", 300): strings.Repeat("a", 200),
	} {
		if got := safeFileName(in); got != want {
			t.Errorf("safeFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReverseOrder(t *testing.T) {
	for _, tt := range []struct {
		sort, order string
		want        bool
	}{
		{"name", "", false}, {"name", "asc", false}, {"name", "desc", true},
		{"updated", "", false}, {"updated", "desc", false}, {"updated", "asc", true},
		{"", "asc", true}, {"size", "asc", true},
	} {
		if got := reverseOrder(tt.sort, tt.order); got != tt.want {
			t.Errorf("reverseOrder(%q, %q) = %v, want %v", tt.sort, tt.order, got, tt.want)
		}
	}
}

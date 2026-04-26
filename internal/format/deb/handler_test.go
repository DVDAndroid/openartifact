// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"strings"
	"testing"

	"github.com/openartifact/openartifact/pkg/deb"
)

func TestDebianHandlerName(t *testing.T) {
	handler := NewDebianHandler()
	if handler.Name() != "deb" {
		t.Errorf("Name() = %v, want deb", handler.Name())
	}
}

func TestDebianHandlerContentType(t *testing.T) {
	handler := NewDebianHandler()
	if handler.GetContentType() != "application/x-deb" {
		t.Errorf("GetContentType() = %v, want application/x-deb", handler.GetContentType())
	}
}

func TestDebianHandlerFileExtension(t *testing.T) {
	handler := NewDebianHandler()
	if handler.GetFileExtension() != ".deb" {
		t.Errorf("GetFileExtension() = %v, want .deb", handler.GetFileExtension())
	}
}

func TestRepositoryHandlerValidatePackage(t *testing.T) {
	handler := NewRepositoryHandler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestRepositoryHandlerGeneratePackagesEntry(t *testing.T) {
	handler := NewRepositoryHandler()

	// Create a mock DebFile
	debFile := &deb.DebFile{
		Package: &deb.Package{
			Package:       "test-package",
			Version:       "1.0.0",
			Architecture:  "amd64",
			Maintainer:    "Test User <test@example.com>",
			Description:   "A test package",
			Section:       "utils",
			Priority:      "optional",
			Homepage:      "https://example.com",
			Depends:       "libc6 (>= 2.17)",
			InstalledSize: 1024,
		},
	}

	entry := handler.GeneratePackagesEntry(debFile, "pool/main/t/test-package/test-package_1.0.0_amd64.deb", 2048, "abc123", "def456", "ghi789")

	if !strings.Contains(entry, "Package: test-package") {
		t.Errorf("Entry missing Package field")
	}

	if !strings.Contains(entry, "Version: 1.0.0") {
		t.Errorf("Entry missing Version field")
	}

	if !strings.Contains(entry, "Architecture: amd64") {
		t.Errorf("Entry missing Architecture field")
	}

	if !strings.Contains(entry, "Filename: pool/main/t/test-package/test-package_1.0.0_amd64.deb") {
		t.Errorf("Entry missing Filename field")
	}

	if !strings.Contains(entry, "Size: 2048") {
		t.Errorf("Entry missing Size field")
	}

	if !strings.Contains(entry, "MD5sum: abc123") {
		t.Errorf("Entry missing MD5sum field")
	}
}

func TestRepositoryHandlerGetPackageInfo(t *testing.T) {
	handler := NewRepositoryHandler()

	debFile := &deb.DebFile{
		Package: &deb.Package{
			Package:      "test-package",
			Version:      "1.0.0",
			Architecture: "amd64",
			Maintainer:   "Test User <test@example.com>",
		},
	}

	info := handler.GetPackageInfo(debFile)

	if info["package"] != "test-package" {
		t.Errorf("Info missing package")
	}

	if info["version"] != "1.0.0" {
		t.Errorf("Info missing version")
	}

	if info["architecture"] != "amd64" {
		t.Errorf("Info missing architecture")
	}
}

func TestRepositoryHandlerGetPackageInfoNil(t *testing.T) {
	handler := NewRepositoryHandler()

	info := handler.GetPackageInfo(nil)
	if len(info) != 0 {
		t.Errorf("GetPackageInfo(nil) should return empty map")
	}
}

func TestGetPoolPath(t *testing.T) {
	tests := []struct {
		name        string
		component   string
		packageName string
		version     string
		filename    string
		want        string
	}{
		{
			name:        "regular package",
			component:   "main",
			packageName: "test-package",
			version:     "1.0.0",
			filename:    "test-package_1.0.0_amd64.deb",
			want:        "pool/main/t/test-package/test-package_1.0.0_amd64.deb",
		},
		{
			name:        "lib package",
			component:   "main",
			packageName: "libtest",
			version:     "1.0.0",
			filename:    "libtest_1.0.0_amd64.deb",
			want:        "pool/main/libt/libtest/libtest_1.0.0_amd64.deb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPoolPath(tt.component, tt.packageName, tt.version, tt.filename)
			if got != tt.want {
				t.Errorf("GetPoolPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPackagesPath(t *testing.T) {
	path := GetPackagesPath("focal", "main", "amd64")
	if path != "dists/focal/main/binary-amd64/Packages" {
		t.Errorf("GetPackagesPath() = %v, want dists/focal/main/binary-amd64/Packages", path)
	}
}

func TestGetPackagesGzPath(t *testing.T) {
	path := GetPackagesGzPath("focal", "main", "amd64")
	if path != "dists/focal/main/binary-amd64/Packages.gz" {
		t.Errorf("GetPackagesGzPath() = %v, want dists/focal/main/binary-amd64/Packages.gz", path)
	}
}

func TestGetReleasePath(t *testing.T) {
	path := GetReleasePath("focal")
	if path != "dists/focal/Release" {
		t.Errorf("GetReleasePath() = %v, want dists/focal/Release", path)
	}
}

func TestGetInReleasePath(t *testing.T) {
	path := GetInReleasePath("focal")
	if path != "dists/focal/InRelease" {
		t.Errorf("GetInReleasePath() = %v, want dists/focal/InRelease", path)
	}
}

func TestGetReleaseGPGPath(t *testing.T) {
	path := GetReleaseGPGPath("focal")
	if path != "dists/focal/Release.gpg" {
		t.Errorf("GetReleaseGPGPath() = %v, want dists/focal/Release.gpg", path)
	}
}

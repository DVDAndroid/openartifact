// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
)

func TestReleaseFileGenerator(t *testing.T) {
	gen := NewReleaseFileGenerator("TestOrigin", "TestLabel", "focal", "20.04")
	gen.AddArchitecture("amd64")
	gen.AddArchitecture("i386")
	gen.AddComponent("main")
	gen.AddComponent("universe")

	files := []FileInfo{
		{
			Filename: "dists/focal/main/binary-amd64/Packages.gz",
			Size:     1024,
			MD5sum:   "abc123",
			SHA1:     "def456",
			SHA256:   "ghi789",
		},
	}

	release := gen.GenerateRelease(files)

	if !strings.Contains(release, "Origin: TestOrigin") {
		t.Errorf("Release file missing Origin")
	}

	if !strings.Contains(release, "Suite: focal") {
		t.Errorf("Release file missing Suite")
	}

	if !strings.Contains(release, "amd64") {
		t.Errorf("Release file missing architecture")
	}

	if !strings.Contains(release, "main") {
		t.Errorf("Release file missing component")
	}

	if !strings.Contains(release, "MD5Sum:") {
		t.Errorf("Release file missing MD5Sum section")
	}
}

func TestPackagesFileGenerator(t *testing.T) {
	gen := NewPackagesFileGenerator()

	entry1 := "Package: test1\nVersion: 1.0\n\n"
	entry2 := "Package: test2\nVersion: 2.0\n\n"

	gen.AddEntry(entry1)
	gen.AddEntry(entry2)

	content := gen.Generate()

	if !strings.Contains(content, "Package: test1") {
		t.Errorf("Packages file missing test1")
	}

	if !strings.Contains(content, "Package: test2") {
		t.Errorf("Packages file missing test2")
	}
}

func TestPackagesFileGeneratorGzipped(t *testing.T) {
	gen := NewPackagesFileGenerator()
	gen.AddEntry("Package: test\nVersion: 1.0\n\n")

	gzipped, err := gen.GenerateGzipped()
	if err != nil {
		t.Fatalf("GenerateGzipped() error = %v", err)
	}

	// Verify it's valid gzip
	gzReader, err := gzip.NewReader(bytes.NewReader(gzipped))
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer func() { _ = gzReader.Close() }()

	data, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("Failed to read gzipped data: %v", err)
	}

	if !strings.Contains(string(data), "Package: test") {
		t.Errorf("Gzipped content missing package")
	}
}

func TestCalculateChecksums(t *testing.T) {
	data := []byte("test data")

	md5sum, sha1sum, sha256sum := CalculateChecksums(data)

	if md5sum == "" {
		t.Errorf("MD5sum is empty")
	}

	if sha1sum == "" {
		t.Errorf("SHA1sum is empty")
	}

	if sha256sum == "" {
		t.Errorf("SHA256sum is empty")
	}

	// Verify they're different
	if md5sum == sha1sum || sha1sum == sha256sum {
		t.Errorf("Checksums should be different")
	}
}

func TestCalculateChecksumsFromReader(t *testing.T) {
	data := []byte("test data for checksums")
	reader := bytes.NewReader(data)

	md5sum, sha1sum, sha256sum, size, err := CalculateChecksumsFromReader(reader)
	if err != nil {
		t.Fatalf("CalculateChecksumsFromReader() error = %v", err)
	}

	if size != int64(len(data)) {
		t.Errorf("Size = %d, want %d", size, len(data))
	}

	if md5sum == "" || sha1sum == "" || sha256sum == "" {
		t.Errorf("One or more checksums are empty")
	}
}

func TestRepositoryIndexGenerator(t *testing.T) {
	gen := NewRepositoryIndexGenerator("TestOrigin", "TestLabel", "focal", "20.04")

	// Add some packages
	pkgGen := gen.GetPackagesGenerator("main", "amd64")
	pkgGen.AddEntry("Package: test1\nVersion: 1.0\n\n")

	pkgGen = gen.GetPackagesGenerator("main", "i386")
	pkgGen.AddEntry("Package: test2\nVersion: 2.0\n\n")

	files, err := gen.GenerateIndexFiles()
	if err != nil {
		t.Fatalf("GenerateIndexFiles() error = %v", err)
	}

	// Check that files were generated
	if _, ok := files["dists/Release"]; !ok {
		t.Errorf("Release file not generated")
	}

	if _, ok := files["dists/main/amd64/Packages"]; !ok {
		t.Errorf("Packages file for main/amd64 not generated")
	}

	if _, ok := files["dists/main/amd64/Packages.gz"]; !ok {
		t.Errorf("Packages.gz file for main/amd64 not generated")
	}

	// Verify Release file content
	releaseContent := string(files["dists/Release"])
	if !strings.Contains(releaseContent, "Origin: TestOrigin") {
		t.Errorf("Release file missing Origin")
	}

	if !strings.Contains(releaseContent, "amd64") {
		t.Errorf("Release file missing architecture")
	}

	if !strings.Contains(releaseContent, "main") {
		t.Errorf("Release file missing component")
	}
}

func TestReleaseFileGeneratorAddDuplicates(t *testing.T) {
	gen := NewReleaseFileGenerator("Origin", "Label", "suite", "codename")

	gen.AddArchitecture("amd64")
	gen.AddArchitecture("amd64") // duplicate
	gen.AddComponent("main")
	gen.AddComponent("main") // duplicate

	if len(gen.Architectures) != 1 {
		t.Errorf("Duplicate architecture added")
	}

	if len(gen.Components) != 1 {
		t.Errorf("Duplicate component added")
	}
}

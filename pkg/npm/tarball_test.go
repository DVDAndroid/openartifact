// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"
)

// validateTarballError parses a tarball and returns only the error.
func validateTarballError(r *bytes.Reader) error {
	manifest, integrity, sha512, err := ParseTarball(r)
	_ = manifest
	_ = integrity
	_ = sha512

	return err
}

func buildTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar WriteHeader: %v", err)
		}

		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar Write: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}

	if err := gw.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}

	return buf.Bytes()
}

func TestParseTarball_Valid(t *testing.T) {
	pkgJSON, _ := json.Marshal(map[string]any{"name": "test-pkg", "version": "1.0.0"})
	tb := buildTarball(t, map[string]string{
		"package/package.json": string(pkgJSON),
	})

	manifest, integrity, shasum, err := ParseTarball(bytes.NewReader(tb))
	if err != nil {
		t.Fatalf("ParseTarball error: %v", err)
	}

	if manifest["name"] != "test-pkg" {
		t.Errorf("manifest name = %v, want test-pkg", manifest["name"])
	}

	if !strings.HasPrefix(integrity, "sha512-") {
		t.Errorf("integrity %q does not start with sha512-", integrity)
	}

	if len(shasum) != 40 {
		t.Errorf("shasum length = %d, want 40", len(shasum))
	}

	for _, ch := range shasum {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			t.Errorf("shasum %q contains non-hex char %c", shasum, ch)
			break
		}
	}
}

func TestParseTarball_MissingPackageJSON(t *testing.T) {
	tb := buildTarball(t, map[string]string{
		"package/README.md": "hello",
	})

	err := validateTarballError(bytes.NewReader(tb))
	if err == nil {
		t.Fatal("expected error for missing package/package.json")
	}

	if !strings.Contains(err.Error(), "package/package.json") {
		t.Errorf("error %q does not mention package/package.json", err.Error())
	}
}

func TestParseTarball_WrongPrefix(t *testing.T) {
	pkgJSON, _ := json.Marshal(map[string]any{"name": "test-pkg", "version": "1.0.0"})
	tb := buildTarball(t, map[string]string{
		"package.json": string(pkgJSON),
	})

	err := validateTarballError(bytes.NewReader(tb))
	if err == nil {
		t.Fatal("expected error when package.json is at archive root")
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseControlFile(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		checks  func(*ControlFile) bool
	}{
		{
			name: "simple control file",
			input: `Package: test-package
Version: 1.0.0
Architecture: amd64
Maintainer: Test User <test@example.com>
Description: A test package

`,
			wantErr: false,
			checks: func(cf *ControlFile) bool {
				return cf.Get("Package") == "test-package" &&
					cf.Get("Version") == "1.0.0" &&
					cf.Get("Architecture") == "amd64"
			},
		},
		{
			name: "control file with continuation lines",
			input: `Package: test-package
Version: 1.0.0
Architecture: amd64
Description: A test package
 with multiple lines
 of description

`,
			wantErr: false,
			checks: func(cf *ControlFile) bool {
				desc := cf.Get("Description")
				return strings.Contains(desc, "multiple lines")
			},
		},
		{
			name: "control file with dependencies",
			input: `Package: test-package
Version: 1.0.0
Architecture: amd64
Depends: libc6 (>= 2.17), libssl1.1 (>= 1.1.0)
Recommends: curl

`,
			wantErr: false,
			checks: func(cf *ControlFile) bool {
				deps := cf.Get("Depends")
				return strings.Contains(deps, "libc6")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf, err := ParseControlFile(bytes.NewReader([]byte(tt.input)))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseControlFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && !tt.checks(cf) {
				t.Errorf("ParseControlFile() checks failed")
			}
		})
	}
}

func TestControlFileGetRequired(t *testing.T) {
	cf := &ControlFile{
		Fields: map[string]string{
			"Package": "test",
			"Version": "1.0",
		},
	}

	tests := []struct {
		name    string
		field   string
		wantErr bool
	}{
		{"existing field", "Package", false},
		{"missing field", "Missing", true},
		{"empty field", "Empty", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cf.GetRequired(tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRequired() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParsePackage(t *testing.T) {
	input := `Package: test-package
Version: 1.0.0
Architecture: amd64
Maintainer: Test User <test@example.com>
Description: A test package
Section: utils
Priority: optional
Homepage: https://example.com

`

	cf, err := ParseControlFile(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseControlFile() error = %v", err)
	}

	pkg, err := cf.ParsePackage()
	if err != nil {
		t.Fatalf("ParsePackage() error = %v", err)
	}

	if pkg.Package != "test-package" {
		t.Errorf("Package = %v, want test-package", pkg.Package)
	}

	if pkg.Version != "1.0.0" {
		t.Errorf("Version = %v, want 1.0.0", pkg.Version)
	}

	if pkg.Architecture != "amd64" {
		t.Errorf("Architecture = %v, want amd64", pkg.Architecture)
	}

	if pkg.Section != "utils" {
		t.Errorf("Section = %v, want utils", pkg.Section)
	}
}

func TestParsePackageRequired(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "missing package",
			input: `Version: 1.0.0
Architecture: amd64

`,
			wantErr: true,
		},
		{
			name: "missing version",
			input: `Package: test
Architecture: amd64

`,
			wantErr: true,
		},
		{
			name: "missing architecture",
			input: `Package: test
Version: 1.0.0

`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf, err := ParseControlFile(bytes.NewReader([]byte(tt.input)))
			if err != nil {
				t.Fatalf("ParseControlFile() error = %v", err)
			}

			_, err = cf.ParsePackage()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePackage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

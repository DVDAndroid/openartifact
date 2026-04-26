// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"testing"
)

func TestParseDependencies(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		checks  func(*DependencyList) bool
	}{
		{
			name:    "empty dependencies",
			input:   "",
			wantErr: false,
			checks: func(dl *DependencyList) bool {
				return len(dl.Groups) == 0
			},
		},
		{
			name:    "single dependency",
			input:   "libc6",
			wantErr: false,
			checks: func(dl *DependencyList) bool {
				return len(dl.Groups) == 1 &&
					len(dl.Groups[0].Dependencies) == 1 &&
					dl.Groups[0].Dependencies[0].Package == "libc6"
			},
		},
		{
			name:    "dependency with version",
			input:   "libc6 (>= 2.17)",
			wantErr: false,
			checks: func(dl *DependencyList) bool {
				dep := dl.Groups[0].Dependencies[0]

				return dep.Package == "libc6" &&
					dep.Operator == ">=" &&
					dep.Version == "2.17"
			},
		},
		{
			name:    "multiple dependencies (AND)",
			input:   "libc6 (>= 2.17), libssl1.1 (>= 1.1.0)",
			wantErr: false,
			checks: func(dl *DependencyList) bool {
				return len(dl.Groups) == 2 &&
					dl.Groups[0].Dependencies[0].Package == "libc6" &&
					dl.Groups[1].Dependencies[0].Package == "libssl1.1"
			},
		},
		{
			name:    "alternative dependencies (OR)",
			input:   "libssl1.1 (>= 1.1.0) | libssl1.0.2",
			wantErr: false,
			checks: func(dl *DependencyList) bool {
				return len(dl.Groups) == 1 &&
					len(dl.Groups[0].Dependencies) == 2 &&
					dl.Groups[0].Dependencies[0].Package == "libssl1.1" &&
					dl.Groups[0].Dependencies[1].Package == "libssl1.0.2"
			},
		},
		{
			name:    "complex dependencies",
			input:   "libc6 (>= 2.17), libssl1.1 (>= 1.1.0) | libssl1.0.2, curl",
			wantErr: false,
			checks: func(dl *DependencyList) bool {
				return len(dl.Groups) == 3
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dl, err := ParseDependencies(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && !tt.checks(dl) {
				t.Errorf("ParseDependencies() checks failed")
			}
		})
	}
}

func TestParseSingleDependency(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		checks  func(*Dependency) bool
	}{
		{
			name:    "simple package",
			input:   "libc6",
			wantErr: false,
			checks: func(d *Dependency) bool {
				return d.Package == "libc6" && d.Operator == ""
			},
		},
		{
			name:    "package with version",
			input:   "libc6 (>= 2.17)",
			wantErr: false,
			checks: func(d *Dependency) bool {
				return d.Package == "libc6" &&
					d.Operator == ">=" &&
					d.Version == "2.17"
			},
		},
		{
			name:    "package with architecture",
			input:   "libc6:amd64",
			wantErr: false,
			checks: func(d *Dependency) bool {
				return d.Package == "libc6" && d.Arch == "amd64"
			},
		},
		{
			name:    "package with version and architecture",
			input:   "libc6:amd64 (>= 2.17)",
			wantErr: false,
			checks: func(d *Dependency) bool {
				return d.Package == "libc6" &&
					d.Arch == "amd64" &&
					d.Operator == ">=" &&
					d.Version == "2.17"
			},
		},
		{
			name:    "invalid format",
			input:   "invalid (format",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := parseSingleDependency(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSingleDependency() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && !tt.checks(d) {
				t.Errorf("parseSingleDependency() checks failed")
			}
		})
	}
}

func TestDependencyString(t *testing.T) {
	tests := []struct {
		name string
		dep  *Dependency
		want string
	}{
		{
			name: "simple package",
			dep:  &Dependency{Package: "libc6"},
			want: "libc6",
		},
		{
			name: "package with version",
			dep: &Dependency{
				Package:  "libc6",
				Operator: ">=",
				Version:  "2.17",
			},
			want: "libc6 (>= 2.17)",
		},
		{
			name: "package with architecture",
			dep: &Dependency{
				Package: "libc6",
				Arch:    "amd64",
			},
			want: "libc6:amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dep.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencyListGetPackages(t *testing.T) {
	dl := &DependencyList{
		Groups: []*DependencyGroup{
			{
				Dependencies: []*Dependency{
					{Package: "libc6"},
					{Package: "libssl1.1"},
				},
			},
			{
				Dependencies: []*Dependency{
					{Package: "curl"},
					{Package: "libc6"}, // duplicate
				},
			},
		},
	}

	pkgs := dl.GetPackages()
	if len(pkgs) != 3 {
		t.Errorf("GetPackages() returned %d packages, want 3", len(pkgs))
	}

	// Check that duplicates are removed
	seen := make(map[string]bool)
	for _, pkg := range pkgs {
		if seen[pkg] {
			t.Errorf("GetPackages() returned duplicate package: %s", pkg)
		}

		seen[pkg] = true
	}
}

func TestIsSatisfied(t *testing.T) {
	tests := []struct {
		name    string
		dep     *Dependency
		version string
		want    bool
	}{
		{
			name:    "no version constraint",
			dep:     &Dependency{Package: "libc6"},
			version: "2.17",
			want:    true,
		},
		{
			name: "exact version match",
			dep: &Dependency{
				Package:  "libc6",
				Operator: "=",
				Version:  "2.17",
			},
			version: "2.17",
			want:    true,
		},
		{
			name: "greater than or equal",
			dep: &Dependency{
				Package:  "libc6",
				Operator: ">=",
				Version:  "2.17",
			},
			version: "2.18",
			want:    true,
		},
		{
			name: "less than",
			dep: &Dependency{
				Package:  "libc6",
				Operator: "<<",
				Version:  "2.17",
			},
			version: "2.16",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.dep.IsSatisfied(tt.version); got != tt.want {
				t.Errorf("IsSatisfied() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{name: "tilde sorts before release", v1: "1.0~rc1", v2: "1.0", want: -1},
		{name: "tilde reverse", v1: "1.0", v2: "1.0~rc1", want: 1},
		{name: "tilde before empty string", v1: "1.0~~", v2: "1.0~", want: -1},
		{name: "epoch dominates upstream", v1: "2:1.0", v2: "10.0", want: 1},
		{name: "epoch dominates upstream reverse", v1: "10.0", v2: "2:1.0", want: -1},
		{name: "missing epoch defaults to zero", v1: "1.0", v2: "0:1.0", want: 0},
		{name: "equal versions", v1: "1.2.3-1", v2: "1.2.3-1", want: 0},
		{name: "numeric segment length beats lexical", v1: "1.2", v2: "1.10", want: -1},
		{name: "debian revision breaks tie", v1: "1.0-2", v2: "1.0-10", want: -1},
		{name: "letter suffix sorts after release", v1: "1.0a", v2: "1.0", want: 1},
		{name: "leading zeros stripped from digit runs", v1: "1.01", v2: "1.1", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareVersions(tt.v1, tt.v2); got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

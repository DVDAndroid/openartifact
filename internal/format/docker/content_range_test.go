// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package docker

import (
	"errors"
	"testing"
)

func TestParseContentRange(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{name: "simple range", header: "0-1023", wantStart: 0, wantEnd: 1023},
		{name: "mid-stream range", header: "1024-2047", wantStart: 1024, wantEnd: 2047},
		{name: "single byte", header: "5-5", wantStart: 5, wantEnd: 5},
		{name: "empty header", header: "", wantErr: true},
		{name: "missing dash", header: "1024", wantErr: true},
		{name: "non-numeric start", header: "x-100", wantErr: true},
		{name: "non-numeric end", header: "0-x", wantErr: true},
		{name: "end before start", header: "100-0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseContentRange(tt.header)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseContentRange(%q): expected error, got start=%d end=%d", tt.header, start, end)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseContentRange(%q): unexpected error: %v", tt.header, err)
			}

			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("parseContentRange(%q) = (%d, %d), want (%d, %d)", tt.header, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestValidateChunkOffset(t *testing.T) {
	tests := []struct {
		name          string
		currentOffset int64
		start         int64
		wantErr       bool
	}{
		{name: "contiguous at zero", currentOffset: 0, start: 0},
		{name: "contiguous mid-stream", currentOffset: 1024, start: 1024},
		{name: "gap: start ahead of offset", currentOffset: 100, start: 200, wantErr: true},
		{name: "overlap: start behind offset", currentOffset: 200, start: 100, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChunkOffset(tt.currentOffset, tt.start)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateChunkOffset(%d, %d): expected error, got nil", tt.currentOffset, tt.start)
				}

				if !errors.Is(err, ErrChunkOffsetMismatch) {
					t.Fatalf("validateChunkOffset(%d, %d): error %v does not wrap ErrChunkOffsetMismatch", tt.currentOffset, tt.start, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("validateChunkOffset(%d, %d): unexpected error: %v", tt.currentOffset, tt.start, err)
			}
		})
	}
}

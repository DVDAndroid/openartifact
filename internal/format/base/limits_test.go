// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package base

import (
	"errors"
	"strings"
	"testing"
)

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadAllLimitedExactLimit(t *testing.T) {
	data, err := ReadAllLimited(strings.NewReader("test"), 4)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test" {
		t.Fatalf("data = %q, want %q", data, "test")
	}
}

func TestReadAllLimitedRejectsContentOverLimit(t *testing.T) {
	_, err := ReadAllLimited(strings.NewReader("test"), 3)
	if !errors.Is(err, ErrContentTooLarge) {
		t.Fatalf("error = %v, want ErrContentTooLarge", err)
	}
}

func TestReadAllLimitedPreservesReaderError(t *testing.T) {
	boom := errors.New("boom")
	_, err := ReadAllLimited(errorReader{boom}, 1)
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want %v", err, boom)
	}
}

func TestReadAllLimitedRejectsNegativeLimit(t *testing.T) {
	_, err := ReadAllLimited(strings.NewReader(""), -1)
	if err == nil {
		t.Fatal("expected error")
	}
}

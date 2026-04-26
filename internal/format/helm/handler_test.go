// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"strings"
	"testing"
)

func createHelmChart(t *testing.T, chartYAML string) []byte {
	t.Helper()

	var buf bytes.Buffer

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	data := []byte(chartYAML)
	_ = tw.WriteHeader(&tar.Header{Name: "mychart/Chart.yaml", Mode: 0644, Size: int64(len(data))})
	_, _ = tw.Write(data)
	_ = tw.Close()
	_ = gw.Close()

	return buf.Bytes()
}

const sampleChartYAML = `apiVersion: v2
name: mychart
version: 1.0.0
description: A test chart
`

func TestHelmHandler_Meta(t *testing.T) {
	h := NewHelmHandler()
	if h.Name() != "helm" {
		t.Errorf("Name() = %q, want helm", h.Name())
	}

	if h.GetFileExtension() != ".tgz" {
		t.Errorf("GetFileExtension() = %q, want .tgz", h.GetFileExtension())
	}
}

func TestHelmHandler_Validate(t *testing.T) {
	h := NewHelmHandler()
	ctx := context.Background()

	t.Run("valid chart", func(t *testing.T) {
		data := createHelmChart(t, sampleChartYAML)
		if err := h.Validate(ctx, bytes.NewReader(data)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid: not gzip", func(t *testing.T) {
		if err := h.Validate(ctx, strings.NewReader("garbage")); err == nil {
			t.Error("expected error for invalid chart")
		}
	})
}

func TestHelmHandler_Parse(t *testing.T) {
	h := NewHelmHandler()
	ctx := context.Background()

	data := createHelmChart(t, sampleChartYAML)

	result, err := h.Parse(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["name"] != "mychart" {
		t.Errorf("name = %v, want mychart", result["name"])
	}

	if result["version"] != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", result["version"])
	}
}

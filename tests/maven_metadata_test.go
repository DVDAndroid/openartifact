// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration_test

import (
	"bytes"
	"testing"
	"time"

	fmtmaven "github.com/openartifact/openartifact/internal/format/maven"
	pkgmaven "github.com/openartifact/openartifact/pkg/maven"
)

func TestGenerateGroupMetadataXML(t *testing.T) {
	fixedTime := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	versions := []string{"1.0.0", "1.1.0", "2.0.0-SNAPSHOT"}

	data, err := fmtmaven.GenerateGroupMetadataXML("com.example", "myapp", versions, "1.1.0", "2.0.0-SNAPSHOT", fixedTime)
	if err != nil {
		t.Fatalf("GenerateGroupMetadataXML error: %v", err)
	}

	mustContain := []string{
		"<groupId>com.example</groupId>",
		"<artifactId>myapp</artifactId>",
		"<version>1.0.0</version>",
		"<version>1.1.0</version>",
		"<version>2.0.0-SNAPSHOT</version>",
		"<release>1.1.0</release>",
		"<latest>2.0.0-SNAPSHOT</latest>",
		"<lastUpdated>" + fixedTime.Format("20060102150405") + "</lastUpdated>",
	}
	for _, want := range mustContain {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("output missing %q\ngot:\n%s", want, data)
		}
	}
}

func TestGenerateSnapshotMetadataXML(t *testing.T) {
	coord := pkgmaven.Coordinate{
		GroupID:    "com.example",
		ArtifactID: "myapp",
		Version:    "1.0.0-SNAPSHOT",
	}
	entries := []fmtmaven.SnapshotEntry{
		{
			Extension: "jar",
			Value:     "1.0.0-20260426.120000-3",
			Updated:   "20260426120000",
		},
	}
	fixedTime := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	data, err := fmtmaven.GenerateSnapshotMetadataXML(coord, "20260426.120000", 3, fixedTime, entries)
	if err != nil {
		t.Fatalf("GenerateSnapshotMetadataXML error: %v", err)
	}

	mustContain := []string{
		"<timestamp>20260426.120000</timestamp>",
		"<buildNumber>3</buildNumber>",
		"<extension>jar</extension>",
		"<value>1.0.0-20260426.120000-3</value>",
		"<updated>20260426120000</updated>",
	}
	for _, want := range mustContain {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("output missing %q\ngot:\n%s", want, data)
		}
	}
}

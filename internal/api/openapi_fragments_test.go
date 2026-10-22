// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"testing"
)

func TestMergeOpenAPIFragments(t *testing.T) {
	base := []byte(`{"openapi":"3.0.3","paths":{"/a":{"get":{}}},"components":{"schemas":{"A":{}}},"tags":[{"name":"T"}]}`)
	frag := []byte(`{"paths":{"/b":{"get":{}}},"components":{"schemas":{"B":{}}},"tags":[{"name":"U"}]}`)

	var doc struct {
		OpenAPI    string                     `json:"openapi"`
		Paths      map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}

	if err := json.Unmarshal(mergeOpenAPIFragments(base, frag), &doc); err != nil {
		t.Fatal(err)
	}

	if doc.OpenAPI != "3.0.3" || len(doc.Paths) != 2 || len(doc.Components.Schemas) != 2 || len(doc.Tags) != 2 {
		t.Fatalf("merged = %+v", doc)
	}
}

func TestMergeOpenAPIFragmentsRefusesRedefinition(t *testing.T) {
	base := []byte(`{"paths":{"/a":{}},"components":{"schemas":{"A":{}}}}`)

	for name, frag := range map[string]string{
		"path":   `{"paths":{"/a":{}}}`,
		"schema": `{"components":{"schemas":{"A":{}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("a redefinition was accepted")
				}
			}()

			mergeOpenAPIFragments(base, []byte(frag))
		})
	}
}

func TestServedSpecIncludesAndroid(t *testing.T) {
	var doc struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}

	if err := json.Unmarshal(openapiSpec, &doc); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"/api/v1/repositories/{name}/apks", "/api/v1/repositories/{name}/debs"} {
		if _, ok := doc.Paths[p]; !ok {
			t.Errorf("served spec lacks %s", p)
		}
	}
}

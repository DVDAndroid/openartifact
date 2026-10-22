// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"fmt"
)

// openAPIFragment is the part of a spec a package contributes: operations,
// schemas and tags.
type openAPIFragment struct {
	Paths      map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	} `json:"components"`
	Tags []json.RawMessage `json:"tags"`
}

// mergeOpenAPIFragments adds the paths, schemas and tags of each fragment to
// the base spec. A fragment that redefines a path or schema is a programming
// error, so it panics at start-up (and in every test of this package) instead
// of silently shadowing documented API.
func mergeOpenAPIFragments(base []byte, fragments ...[]byte) []byte {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(base, &doc); err != nil {
		panic(fmt.Sprintf("openapi.json: %v", err))
	}

	var (
		paths   map[string]json.RawMessage
		comps   map[string]json.RawMessage
		schemas map[string]json.RawMessage
		tags    []json.RawMessage
	)

	decode := func(name string, raw json.RawMessage, dst any) {
		if len(raw) == 0 {
			return
		}

		if err := json.Unmarshal(raw, dst); err != nil {
			panic(fmt.Sprintf("openapi.json %s: %v", name, err))
		}
	}

	decode("paths", doc["paths"], &paths)
	decode("components", doc["components"], &comps)
	decode("components.schemas", comps["schemas"], &schemas)
	decode("tags", doc["tags"], &tags)

	for _, raw := range fragments {
		var f openAPIFragment
		if err := json.Unmarshal(raw, &f); err != nil {
			panic(fmt.Sprintf("openapi fragment: %v", err))
		}

		for p, v := range f.Paths {
			if _, dup := paths[p]; dup {
				panic("openapi fragment redefines path " + p)
			}

			paths[p] = v
		}

		for n, v := range f.Components.Schemas {
			if _, dup := schemas[n]; dup {
				panic("openapi fragment redefines schema " + n)
			}

			schemas[n] = v
		}

		tags = append(tags, f.Tags...)
	}

	encode := func(v any) json.RawMessage {
		b, err := json.Marshal(v)
		if err != nil {
			panic(fmt.Sprintf("openapi: %v", err))
		}

		return b
	}

	comps["schemas"] = encode(schemas)
	doc["components"] = encode(comps)
	doc["paths"] = encode(paths)

	if len(tags) > 0 {
		doc["tags"] = encode(tags)
	}

	out, err := json.Marshal(doc)
	if err != nil {
		panic(fmt.Sprintf("openapi: %v", err))
	}

	return out
}

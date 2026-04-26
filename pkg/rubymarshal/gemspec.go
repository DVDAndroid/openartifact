// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rubymarshal

import (
	"fmt"
	"strings"
)

// Dependency is one entry of Gem::Specification#dependencies.
type Dependency struct {
	Name        string
	Requirement string // e.g. ">= 1.0, < 3.0"; comma-joined requirement clauses
	Type        string // "runtime" (default) or "development"
}

// Specification is the subset of Gem::Specification this package round-trips:
// the fields RubyGems' legacy quick/Marshal.4.8 gemspec endpoint and the
// upload metadata.gz payload actually need (name, version, platform,
// dependencies, summary, authors, licenses), plus a few string fields that
// travel for free.
//
// Deliberately NOT modeled (documented scope): the release date
// (a Time — Marshal-dumping Time exactly as MRI does is out of scope, so it
// round-trips as unset/nil), required_ruby_version / required_rubygems_version
// content (RubyGem's model has no such fields; a permissive ">= 0" is always
// emitted), the free-form metadata Hash (always empty), and non-"ruby"
// platforms as a real Gem::Platform object (emitted/read as a plain string
// instead — rubygems' Specification#platform= accepts that fine).
type Specification struct {
	Name         string
	Version      string
	Platform     string // "ruby" for pure-Ruby gems; defaults to "ruby" if empty
	Summary      string
	Description  string
	Homepage     string
	Email        string
	Authors      []string
	Licenses     []string
	Dependencies []Dependency
}

const (
	rubygemsVersion      = "3.4.10"
	specificationVersion = 4
)

// NewGemVersion returns the Marshal representation of Gem::Version.new(v):
// Gem::Version#marshal_dump returns [@version], a one-element array holding
// the raw version string.
func NewGemVersion(v string) UserMarshal {
	return UserMarshal{Class: "Gem::Version", Payload: []any{v}}
}

// EncodeSpecification renders spec as the Marshal 4.8 byte layout RubyGems
// uses for an individual gemspec (the payload of quick/Marshal.4.8/*.gemspec.rz,
// before zlib deflation): a Marshal UserDefined ('u') wrapper named
// "Gem::Specification" around a nested, fully independent Marshal stream
// holding the 19-element array Gem::Specification#_dump/.self._load exchange.
// See https://github.com/rubygems/rubygems/blob/master/lib/rubygems/specification.rb
// for the field order this mirrors — it is ABI-locked (comment there reads
// "MAINTAIN ORDER!") so this layout is stable across RubyGems versions.
func EncodeSpecification(spec Specification) ([]byte, error) {
	platform := spec.Platform
	if platform == "" {
		platform = "ruby"
	}

	deps := make([]any, len(spec.Dependencies))

	for i, dep := range spec.Dependencies {
		depType := dep.Type
		if depType == "" {
			depType = "runtime"
		}

		req, err := encodeRequirement(dep.Requirement)
		if err != nil {
			return nil, fmt.Errorf("rubymarshal: dependency %q: %w", dep.Name, err)
		}

		deps[i] = Object{
			Class: "Gem::Dependency",
			IVars: []IVar{
				{Name: "name", Value: dep.Name},
				{Name: "requirement", Value: req},
				{Name: "type", Value: Symbol(depType)},
				{Name: "prerelease", Value: false},
				{Name: "version_requirements", Value: req}, // legacy ivar, same value
			},
		}
	}

	defaultReq, err := encodeRequirement("")
	if err != nil {
		return nil, err
	}

	authors := make([]any, len(spec.Authors))
	for i, a := range spec.Authors {
		authors[i] = a
	}

	licenses := make([]any, len(spec.Licenses))
	for i, l := range spec.Licenses {
		licenses[i] = l
	}

	array := []any{
		rubygemsVersion,             // 0  @rubygems_version
		int64(specificationVersion), // 1  @specification_version
		spec.Name,                   // 2  @name
		NewGemVersion(spec.Version), // 3  @version
		nil,                         // 4  date (out of scope, see doc comment)
		spec.Summary,                // 5  @summary
		defaultReq,                  // 6  @required_ruby_version
		defaultReq,                  // 7  @required_rubygems_version
		platform,                    // 8  @original_platform
		deps,                        // 9  @dependencies
		"",                          // 10 rubyforge_project (removed, kept for ABI)
		spec.Email,                  // 11 @email
		authors,                     // 12 @authors
		spec.Description,            // 13 @description
		spec.Homepage,               // 14 @homepage
		true,                        // 15 has_rdoc (removed, kept for ABI)
		platform,                    // 16 @new_platform
		licenses,                    // 17 @licenses
		Hash{},                      // 18 @metadata
	}

	inner, err := Encode(array)
	if err != nil {
		return nil, fmt.Errorf("rubymarshal: encode gemspec array: %w", err)
	}

	return Encode(UserDefined{Class: "Gem::Specification", Data: inner})
}

// DecodeSpecification parses the byte layout EncodeSpecification produces
// (or an equivalent real RubyGems-generated one, within this package's
// documented scope) back into a Specification.
func DecodeSpecification(data []byte) (*Specification, error) {
	root, err := Decode(data)
	if err != nil {
		return nil, err
	}

	ud, ok := root.(*UserDefined)
	if !ok || ud.Class != "Gem::Specification" {
		return nil, fmt.Errorf("%w: not a Gem::Specification payload (got %T)", ErrInvalid, root)
	}

	inner, err := Decode(ud.Data)
	if err != nil {
		return nil, fmt.Errorf("rubymarshal: decode nested gemspec array: %w", err)
	}

	array, ok := inner.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: gemspec payload is not an array (got %T)", ErrInvalid, inner)
	}

	return specificationFromArray(array), nil
}

func specificationFromArray(array []any) *Specification {
	get := func(i int) any {
		if i < len(array) {
			return array[i]
		}

		return nil
	}

	spec := &Specification{
		Name:        asString(get(2)),
		Version:     decodeGemVersion(get(3)),
		Platform:    asString(get(8)),
		Summary:     asString(get(5)),
		Email:       asString(get(11)),
		Description: asString(get(13)),
		Homepage:    asString(get(14)),
		Authors:     asStringSlice(get(12)),
		Licenses:    asStringSlice(get(17)),
	}

	if spec.Platform == "" {
		spec.Platform = "ruby"
	}

	if depsVal, ok := get(9).([]any); ok {
		for _, dv := range depsVal {
			if dep, ok := dependencyFromValue(dv); ok {
				spec.Dependencies = append(spec.Dependencies, dep)
			}
		}
	}

	return spec
}

// encodeRequirement builds a Gem::Requirement UserMarshal from a
// comma-separated list of clauses such as ">= 1.0, < 3.0" or "~> 2.1". A
// blank string is treated as the default (unconstrained) requirement.
func encodeRequirement(requirement string) (UserMarshal, error) {
	clauses := splitRequirement(requirement)
	reqs := make([]any, len(clauses))

	for i, c := range clauses {
		op, ver, err := parseClause(c)
		if err != nil {
			return UserMarshal{}, err
		}

		reqs[i] = []any{op, NewGemVersion(ver)}
	}

	return UserMarshal{Class: "Gem::Requirement", Payload: []any{reqs}}, nil
}

func splitRequirement(requirement string) []string {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return []string{">= 0"}
	}

	parts := strings.Split(requirement, ",")
	clauses := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clauses = append(clauses, p)
		}
	}

	if len(clauses) == 0 {
		return []string{">= 0"}
	}

	return clauses
}

func parseClause(clause string) (string, string, error) {
	clause = strings.TrimSpace(clause)
	if clause == "" {
		return "", "", fmt.Errorf("%w: empty requirement clause", ErrInvalid)
	}

	// Gem::Requirement operators, longest/most-specific first so prefix
	// matching never mistakes ">=" for ">".
	for _, o := range [...]string{">=", "<=", "~>", "!=", ">", "<", "="} {
		if rest, ok := strings.CutPrefix(clause, o); ok {
			return o, strings.TrimSpace(rest), nil
		}
	}

	// A bare version number implies an exact-match requirement.
	return "=", clause, nil
}

func asString(v any) string {
	s, _ := v.(string)

	return s
}

func asStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(arr))

	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}

	return out
}

func decodeGemVersion(v any) string {
	um, ok := v.(*UserMarshal)
	if !ok || um.Class != "Gem::Version" {
		return ""
	}

	arr, ok := um.Payload.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}

	return asString(arr[0])
}

func dependencyFromValue(v any) (Dependency, bool) {
	obj, ok := v.(*Object)
	if !ok || obj.Class != "Gem::Dependency" {
		return Dependency{}, false
	}

	name, _ := obj.IVar("name")
	typ, _ := obj.IVar("type")

	reqVal, hasReq := obj.IVar("requirement")
	if !hasReq {
		reqVal, _ = obj.IVar("version_requirements") // pre-3.x ivar name
	}

	depType := "runtime"
	if sym, ok := typ.(Symbol); ok && sym != "" {
		depType = string(sym)
	}

	return Dependency{
		Name:        asString(name),
		Requirement: decodeRequirement(reqVal),
		Type:        depType,
	}, true
}

func decodeRequirement(v any) string {
	um, ok := v.(*UserMarshal)
	if !ok || um.Class != "Gem::Requirement" {
		return ""
	}

	payload, ok := um.Payload.([]any)
	if !ok || len(payload) == 0 {
		return ""
	}

	clauses, ok := payload[0].([]any)
	if !ok {
		return ""
	}

	parts := make([]string, 0, len(clauses))

	for _, c := range clauses {
		pair, ok := c.([]any)
		if !ok || len(pair) != 2 {
			continue
		}

		op := asString(pair[0])

		ver := decodeGemVersion(pair[1])
		if op == "" || ver == "" {
			continue
		}

		parts = append(parts, op+" "+ver)
	}

	return strings.Join(parts, ", ")
}

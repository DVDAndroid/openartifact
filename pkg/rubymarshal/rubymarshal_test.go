// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rubymarshal

import (
	"math"
	"reflect"
	"testing"
)

func roundTrip(t *testing.T, v any) any {
	t.Helper()

	data, err := Encode(v)
	if err != nil {
		t.Fatalf("Encode(%#v): %v", v, err)
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode(Encode(%#v)): %v", v, err)
	}

	return got
}

func TestRoundTripScalars(t *testing.T) {
	cases := []any{
		nil,
		true,
		false,
		int64(0),
		int64(1),
		int64(-1),
		int64(122),
		int64(123),
		int64(-123),
		int64(-124),
		int64(1000),
		int64(-1000),
		int64(1 << 20),
		int64(-(1 << 20)),
		int64(1 << 30),
		int64(-(1 << 30)),
		"",
		"hello",
		"unicode: héllo wörld 日本語",
		Symbol("runtime"),
	}

	for _, c := range cases {
		got := roundTrip(t, c)
		if !reflect.DeepEqual(got, c) {
			t.Errorf("round trip %#v: got %#v", c, got)
		}
	}
}

func TestEncodeFixnumOverflow(t *testing.T) {
	// Fixnum long-form is capped at 4 bytes (see writeLong); anything wider
	// needs Bignum, which this package intentionally does not encode.
	if _, err := Encode(int64(1) << 40); err == nil {
		t.Error("expected Encode to reject a Fixnum wider than 4 bytes")
	}
}

func TestRoundTripFloat(t *testing.T) {
	cases := []float64{0, 1.5, -1.5, 3.14159, 1e10, -1e-10}

	for _, f := range cases {
		got := roundTrip(t, f)
		gf, ok := got.(float64)
		if !ok {
			t.Fatalf("round trip %v: got %T", f, got)
		}

		if gf != f {
			t.Errorf("round trip float %v: got %v", f, gf)
		}
	}

	for _, f := range []float64{math.NaN()} {
		got := roundTrip(t, f)
		gf, ok := got.(float64)
		if !ok || !math.IsNaN(gf) {
			t.Errorf("round trip NaN: got %#v", got)
		}
	}

	for name, f := range map[string]float64{"+inf": math.Inf(1), "-inf": math.Inf(-1)} {
		got := roundTrip(t, f)
		gf, ok := got.(float64)
		if !ok || gf != f {
			t.Errorf("round trip %s: got %#v", name, got)
		}
	}
}

func TestRoundTripArrayAndHash(t *testing.T) {
	arr := []any{"a", int64(2), []any{Symbol("nested"), nil, true}}

	got := roundTrip(t, arr)
	if !reflect.DeepEqual(got, arr) {
		t.Errorf("array round trip: got %#v, want %#v", got, arr)
	}

	h := Hash{
		{Key: Symbol("a"), Value: int64(1)},
		{Key: "b", Value: []any{int64(2), int64(3)}},
	}

	got = roundTrip(t, h)
	if !reflect.DeepEqual(got, h) {
		t.Errorf("hash round trip: got %#v, want %#v", got, h)
	}
}

func TestRoundTripObject(t *testing.T) {
	obj := Object{
		Class: "Gem::Dependency",
		IVars: []IVar{
			{Name: "name", Value: "rack"},
			{Name: "type", Value: Symbol("runtime")},
			{Name: "prerelease", Value: false},
		},
	}

	got := roundTrip(t, obj)

	gotObj, ok := got.(*Object)
	if !ok {
		t.Fatalf("expected *Object, got %T", got)
	}

	if gotObj.Class != obj.Class {
		t.Errorf("class = %q, want %q", gotObj.Class, obj.Class)
	}

	for _, iv := range obj.IVars {
		v, ok := gotObj.IVar(iv.Name)
		if !ok {
			t.Errorf("missing ivar %q", iv.Name)
			continue
		}

		if !reflect.DeepEqual(v, iv.Value) {
			t.Errorf("ivar %q = %#v, want %#v", iv.Name, v, iv.Value)
		}
	}
}

func TestRoundTripUserMarshalAndUserDefined(t *testing.T) {
	um := NewGemVersion("1.2.3")

	got := roundTrip(t, um)

	gotUM, ok := got.(*UserMarshal)
	if !ok {
		t.Fatalf("expected *UserMarshal, got %T", got)
	}

	if gotUM.Class != "Gem::Version" {
		t.Errorf("class = %q, want Gem::Version", gotUM.Class)
	}

	if decodeGemVersion(gotUM) != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", decodeGemVersion(gotUM))
	}

	ud := UserDefined{Class: "Gem::Specification", Data: []byte{4, 8, '0'}}

	got = roundTrip(t, ud)

	gotUD, ok := got.(*UserDefined)
	if !ok {
		t.Fatalf("expected *UserDefined, got %T", got)
	}

	if gotUD.Class != ud.Class || !reflect.DeepEqual(gotUD.Data, ud.Data) {
		t.Errorf("UserDefined round trip = %#v, want %#v", gotUD, ud)
	}
}

// TestDecodeSymbolAndObjectBackreferences hand-builds the byte stream real
// Ruby emits for Marshal.dump([:foo, :foo]) — the second :foo is a symbol
// link (';'), not a fresh symbol — to prove the link table is honored.
func TestDecodeSymbolAndObjectBackreferences(t *testing.T) {
	data := []byte{
		4, 8, // version
		'[', 2 + 5, // array, count=2
		':', 3 + 5, 'f', 'o', 'o', // :foo (new symbol, index 0)
		';', 0, // symbol link -> index 0
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("expected 2-element array, got %#v", got)
	}

	for i, v := range arr {
		sym, ok := v.(Symbol)
		if !ok || sym != "foo" {
			t.Errorf("element %d = %#v, want Symbol(foo)", i, v)
		}
	}

	// Now an object backreference: encode a single Gem::Version-shaped
	// Object, splice it in as element 0 of a hand-built 2-element array, and
	// make element 1 an '@' link to that object's link-table slot — this is
	// exactly how real Ruby dedupes repeated Gem::Dependency/Gem::Version
	// instances in a gemspec dump.
	obj := Object{Class: "Gem::Version", IVars: []IVar{{Name: "version", Value: "1.0"}}}

	objBytes, err := Encode(obj)
	if err != nil {
		t.Fatalf("Encode obj: %v", err)
	}

	// Link-table slots, in allocation order: 0 = outer array (reserved
	// before its elements are decoded), 1 = obj itself, 2 = obj's "1.0"
	// string ivar value. Element 1 below must link to slot 1.
	dataObj := []byte{4, 8, '[', 2 + 5}
	dataObj = append(dataObj, objBytes[2:]...) // element 0: the object, verbatim
	dataObj = append(dataObj, '@', 1+5)        // element 1: link -> slot 1 (obj)

	got, err = Decode(dataObj)
	if err != nil {
		t.Fatalf("Decode object backreference: %v", err)
	}

	outer, ok := got.([]any)
	if !ok || len(outer) != 2 {
		t.Fatalf("expected 2-element outer array, got %#v", got)
	}

	first, ok := outer[0].(*Object)
	if !ok {
		t.Fatalf("element 0 = %#v, want *Object", outer[0])
	}

	second, ok := outer[1].(*Object)
	if !ok {
		t.Fatalf("element 1 = %#v, want *Object (via backreference)", outer[1])
	}

	if first != second {
		t.Errorf("element 1 should backreference the exact same *Object as element 0")
	}

	if v, _ := second.IVar("version"); v != "1.0" {
		t.Errorf("backreferenced object version = %#v, want 1.0", v)
	}
}

func TestDecodeRejectsNonMarshalData(t *testing.T) {
	if _, err := Decode([]byte(`{"name":"rails"}`)); err == nil {
		t.Error("expected error decoding non-Marshal (JSON) bytes")
	}

	if _, err := Decode([]byte{4, 8}); err == nil {
		t.Error("expected error decoding truncated stream")
	}
}

// decodeSafely runs Decode and turns any panic into a normal test failure
// instead of a crashed test binary, so a make()-with-hostile-length panic is reported like any other failing
// assertion rather than aborting the whole `go test` run.
func decodeSafely(t *testing.T, data []byte) (any, error) {
	t.Helper()

	var (
		v   any
		err error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Decode(%v) panicked: %v", data, r)
			}
		}()

		v, err = Decode(data)
	}()

	return v, err
}

// TestDecodeRejectsHostileLengths feeds Decode hand-built Marshal streams
// where a length/count field (readLong) has been crafted to be negative or
// ~2^31, and streams that claim a valid count but are truncated right after
// it. Every case must come back as a plain error — never a panic (negative
// length passed to make/slice) and never a multi-GB allocation attempt
// (make([]T, hugeN) before a single element is read). This is the class of
// input RubyGemsHandler.Parse feeds DecodeSpecification straight from an
// uploaded .gem's untrusted metadata.gz.
func TestDecodeRejectsHostileLengths(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		// Negative counts: byte 0x80 is c=-128 (int8), short negative
		// long form, decoding to -123 — readLong can produce this from a
		// single crafted byte.
		{"negative array length", []byte{4, 8, '[', 0x80}},
		{"negative hash length", []byte{4, 8, '{', 0x80}},
		{"negative hash-with-default length", []byte{4, 8, '}', 0x80}},
		{"negative object ivar count", []byte{4, 8, 'o', ':', 3 + 5, 'F', 'o', 'o', 0x80}},
		{"negative string length", []byte{4, 8, '"', 0x80}},
		{"negative symbol length", []byte{4, 8, ':', 0x80}},
		{"negative user-defined length", []byte{4, 8, 'u', ':', 3 + 5, 'F', 'o', 'o', 0x80}},
		{"negative bignum word count", []byte{4, 8, 'l', '+', 0x80}},
		{"negative ivar-wrapped count", []byte{4, 8, 'I', '0', 0x80}},

		// Huge (~2^31) counts: long form c=4 followed by the little-endian
		// bytes of 0x7FFFFFFF (2147483647).
		{"huge (~2^31) array length", []byte{4, 8, '[', 4, 0xFF, 0xFF, 0xFF, 0x7F}},
		{"huge (~2^31) hash length", []byte{4, 8, '{', 4, 0xFF, 0xFF, 0xFF, 0x7F}},
		{"huge (~2^31) object ivar count", []byte{4, 8, 'o', ':', 3 + 5, 'F', 'o', 'o', 4, 0xFF, 0xFF, 0xFF, 0x7F}},
		{"huge (~2^31) string length", []byte{4, 8, '"', 4, 0xFF, 0xFF, 0xFF, 0x7F}},
		{"huge (~2^31) symbol length", []byte{4, 8, ':', 4, 0xFF, 0xFF, 0xFF, 0x7F}},
		{"huge (~2^31) user-defined length", []byte{4, 8, 'u', ':', 3 + 5, 'F', 'o', 'o', 4, 0xFF, 0xFF, 0xFF, 0x7F}},
		{"huge (~2^31) bignum word count", []byte{4, 8, 'l', '+', 4, 0xFF, 0xFF, 0xFF, 0x7F}},

		// Valid, small counts with the underlying reader truncated right
		// after the count field — must fail on the first element read, not
		// hang or panic.
		{"truncated after valid array count", []byte{4, 8, '[', 5 + 5}},
		{"truncated after valid hash count", []byte{4, 8, '{', 3 + 5}},
		{"truncated after valid object ivar count", []byte{4, 8, 'o', ':', 3 + 5, 'F', 'o', 'o', 2 + 5}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeSafely(t, tc.data); err == nil {
				t.Errorf("Decode(%v): expected error, got nil", tc.data)
			}
		})
	}
}

// TestRoundTripLargeArray exercises the append-based (rather than
// make([]any, n)-then-index) rewrite of readArrayTagged with an element
// count well past the 64-element initial capacity hint, to prove the slice
// still grows correctly and every element survives the round trip.
func TestRoundTripLargeArray(t *testing.T) {
	arr := make([]any, 2000)
	for i := range arr {
		arr[i] = int64(i)
	}

	got := roundTrip(t, arr)

	gotArr, ok := got.([]any)
	if !ok || len(gotArr) != len(arr) {
		t.Fatalf("got %#v (len %d), want %d-element array", got, len(gotArr), len(arr))
	}

	for i, v := range gotArr {
		if v != arr[i] {
			t.Fatalf("element %d = %#v, want %#v", i, v, arr[i])
		}
	}
}

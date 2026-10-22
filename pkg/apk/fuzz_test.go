// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"bytes"
	"testing"

	"github.com/openartifact/openartifact/pkg/apk/apktest"
)

func fuzzLimits() Limits {
	lim := DefaultLimits()
	lim.MaxStrings = 10_000
	lim.MaxStringLen = 4096
	lim.MaxXMLNodes = 10_000
	lim.MaxResTypes = 1000

	return lim
}

func FuzzAXML(f *testing.F) {
	f.Add(apktest.EncodeXML(&apktest.XNode{Name: "manifest", Attrs: []apktest.XAttr{
		apktest.AInt("versionCode", apktest.AttrVersionCode, 1),
		{Name: "package", Type: apktest.TypeString, Str: "a.b"},
	}}))

	f.Fuzz(func(_ *testing.T, b []byte) {
		root, ns, err := parseAXML(b, fuzzLimits())
		if err == nil {
			_, _ = manifestFrom(root)
			_ = renderXML(root, ns)
		}
	})
}

func FuzzResTable(f *testing.F) {
	tbl := &apktest.Table{Package: "a.b", Types: []string{"string"}, Entries: map[string][]apktest.ResEntry{
		"string": {{Key: "app_name", Values: []apktest.ResValue{{Type: apktest.TypeString, Str: "App"}, {Config: apktest.ResConfig{Language: "it"}, Type: apktest.TypeString, Str: "App"}}}},
	}}
	f.Add(tbl.Encode())

	f.Fuzz(func(_ *testing.T, b []byte) {
		t, err := parseResTable(b, fuzzLimits())
		if err == nil {
			_ = t.resolveAll(0x7f010000, 0)
			_ = t.resourceName(0x7f010000)
		}
	})
}

func FuzzSigningBlock(f *testing.F) {
	// A small seed: large inputs make the fuzzer's minimization crawl.
	small := &apktest.Builder{Package: "a.b", VersionCode: 1, Labels: map[string]string{"": "A"}, Icons: map[uint16]int{160: 2}}
	f.Add(small.Build(apktest.Sign{Key: apktest.NewECKey("f"), V1: true, V2: true, V3: true}))

	f.Fuzz(func(_ *testing.T, b []byte) {
		_, _ = Inspect(bytes.NewReader(b), int64(len(b)), Options{Limits: fuzzLimits(), SkipIconData: true})
	})
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestValidPackageName(t *testing.T) {
	for name, want := range map[string]bool{
		"org.example.app":        true,
		"com.a_b.c9":             true,
		"single":                 true,
		"":                       false,
		"org..example":           false,
		"org.example/../x":       false,
		"9org.example":           false,
		"org.exa mple":           false,
		strings.Repeat("a", 256): false,
	} {
		if got := validPackageName(name); got != want {
			t.Errorf("validPackageName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestNormalizeTags(t *testing.T) {
	got, err := normalizeTags([]string{" Patched ", "device:Pixel", "patched", ""})
	if err != nil || !slices.Equal(got, []string{"patched", "device:pixel"}) {
		t.Fatalf("normalizeTags = %v, %v", got, err)
	}

	for _, bad := range []string{"has space", "-lead", "semi;colon", strings.Repeat("a", 65), "<b>"} {
		if _, err := normalizeTags([]string{bad}); err == nil {
			t.Errorf("tag %q accepted", bad)
		} else if _, ok := isInputError(err); !ok {
			t.Errorf("tag %q: error %v is not an input error", bad, err)
		}
	}

	many := make([]string, maxTags+1)
	for i := range many {
		many[i] = "t" + strings.Repeat("x", i)
	}

	if _, err := normalizeTags(many); err == nil {
		t.Error("too many tags accepted")
	}
}

func TestVariantKeyIgnoresABIOrder(t *testing.T) {
	a := variantKey("s", []string{"arm64-v8a", "x86"})
	b := variantKey("s", []string{"x86", "arm64-v8a"})

	if a != b {
		t.Error("variant key depends on ABI order")
	}

	if a == variantKey("t", []string{"arm64-v8a", "x86"}) || a == variantKey("s", nil) {
		t.Error("variant key ignores signer or ABIs")
	}

	if len(a) != 64 {
		t.Errorf("variant key length %d, want 64 (column is varchar(64))", len(a))
	}
}

func TestDownloadName(t *testing.T) {
	if got := downloadName("org.example", 12, strings.Repeat("ab", 32)); got != "org.example-12-abababab.apk" {
		t.Errorf("downloadName = %q", got)
	}

	if got := downloadName("org.example", 12, ""); got != "org.example-12-unsigned.apk" {
		t.Errorf("unsigned downloadName = %q", got)
	}
}

func TestSpoolRefusesOversizedInput(t *testing.T) {
	sp, err := spool(bytes.NewReader([]byte("12345678")), 8)
	if err != nil {
		t.Fatal(err)
	}
	defer sp.close()

	if sp.size != 8 || len(sp.sha) != 64 {
		t.Fatalf("spooled = %d bytes, sha %q", sp.size, sp.sha)
	}

	if _, err := spool(bytes.NewReader([]byte("123456789")), 8); !errors.Is(err, errTooLarge) {
		t.Fatalf("err = %v, want errTooLarge", err)
	}
}

func TestCheckFields(t *testing.T) {
	for _, bad := range []string{"ftp://x", "javascript:alert(1)", "http://", "not a url", "/relative"} {
		if checkSourceURL(bad) == nil {
			t.Errorf("source_url %q accepted", bad)
		}
	}

	if checkSourceURL("https://f-droid.org/packages/x") != nil || checkSourceURL("") != nil {
		t.Error("valid source_url rejected")
	}

	if checkPatchInfo([]byte(`[1]`)) == nil || checkPatchInfo([]byte(`null`)) == nil || checkPatchInfo([]byte(`{"tool":"x"}`)) != nil {
		t.Error("patch_info validation is wrong")
	}

	if _, err := normalizeChannel("Beta"); err != nil {
		t.Error(err)
	}

	if _, err := normalizeChannel("bad channel"); err == nil {
		t.Error("channel with a space accepted")
	}
}

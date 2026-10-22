// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"bytes"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/openartifact/openartifact/pkg/apk/apktest"
)

func sampleBuilder() *apktest.Builder {
	return &apktest.Builder{
		Package: "org.example.app", VersionCode: 42, VersionName: "1.2.3", MinSDK: 21, TargetSDK: 34,
		Labels:      map[string]string{"": "Example", "it": "Esempio", "pt-BR": "Exemplo"},
		Icons:       map[uint16]int{160: 48, 480: 144},
		Adaptive:    true,
		Permissions: []string{"android.permission.INTERNET", "android.permission.CAMERA"},
		NativeLibs:  map[string][]string{"arm64-v8a": {"libfoo.so"}, "x86_64": {"libfoo.so"}},
		Files: map[string][]byte{
			"META-INF/androidx.core_core.version":                       []byte("1.13.1\n"),
			"META-INF/com/android/build/gradle/app-metadata.properties": []byte("appMetadataVersion=1.1\nandroidGradlePluginVersion=8.5.0\n"),
			"META-INF/maven/com.squareup.okhttp3/okhttp/pom.properties": []byte("groupId=com.squareup.okhttp3\nartifactId=okhttp\nversion=4.12.0\n"),
			"assets/readme.txt": []byte("hello"),
		},
	}
}

func inspectBytes(t *testing.T, b []byte) *Info {
	t.Helper()

	info, err := Inspect(bytes.NewReader(b), int64(len(b)), Options{})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	return info
}

func TestInspectSynthetic(t *testing.T) {
	rsa, ec := apktest.NewRSAKey("Release"), apktest.NewECKey("Patched")

	cases := []struct {
		name    string
		sign    apktest.Sign
		schemes []string
	}{
		{"v1 only", apktest.Sign{Key: rsa, V1: true}, []string{"v1"}},
		{"v2 ec", apktest.Sign{Key: ec, V2: true}, []string{"v2"}},
		{"v1 v2 v3 rsa", apktest.Sign{Key: rsa, V1: true, V2: true, V3: true}, []string{"v1", "v2", "v3"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := inspectBytes(t, sampleBuilder().Build(tc.sign))

			m := info.Manifest
			if m.Package != "org.example.app" || m.VersionCode != 42 || m.VersionName != "1.2.3" || m.MinSDK != 21 || m.TargetSDK != 34 {
				t.Errorf("manifest = %+v", m)
			}

			if !slices.Equal(info.Signature.Schemes, tc.schemes) || !info.Signature.Verified {
				t.Errorf("schemes = %v verified = %v results = %+v", info.Signature.Schemes, info.Signature.Verified, info.Signature.Results)
			}

			if got := info.SignerSHA256(); got != tc.sign.Key.SHA256() {
				t.Errorf("signer = %s, want %s", got, tc.sign.Key.SHA256())
			}
		})
	}
}

func TestInspectMetadata(t *testing.T) {
	info := inspectBytes(t, sampleBuilder().Build(apktest.Sign{Key: apktest.NewECKey("k"), V2: true}))

	for name, check := range map[string]func(*testing.T, *Info){
		"labels": checkLabels, "icons": checkIcons, "native": checkNative, "deps": checkDeps, "manifest": checkManifest,
	} {
		t.Run(name, func(t *testing.T) { check(t, info) })
	}
}

func checkLabels(t *testing.T, info *Info) {
	t.Helper()

	if info.Label != "Example" || info.Labels["it"] != "Esempio" || info.Labels["pt-BR"] != "Exemplo" {
		t.Errorf("labels = %q %v", info.Label, info.Labels)
	}

	if !slices.Equal(info.Locales, []string{"it", "pt-BR"}) {
		t.Errorf("locales = %v", info.Locales)
	}
}

func checkIcons(t *testing.T, info *Info) {
	t.Helper()

	got := map[string]Icon{}
	for _, ic := range info.Icons {
		got[ic.Role+"/"+ic.Density] = ic
	}

	for key, size := range map[string]int{"app/mdpi": 48, "app/xxhdpi": 144, "foreground/xxxhdpi": 432} {
		ic, ok := got[key]
		if !ok || ic.Width != size || ic.Height != size || ic.Mime != "image/png" || len(ic.Data) == 0 || ic.SHA256 == "" {
			t.Errorf("icon %s = %+v", key, ic)
		}
	}

	if bg := got["background/"]; bg.Color != "#ff3ddc84" || !bg.Adaptive {
		t.Errorf("background = %+v", bg)
	}
}

func checkNative(t *testing.T, info *Info) {
	t.Helper()

	if !slices.Equal(info.ABIs, []string{"arm64-v8a", "x86_64"}) || len(info.NativeLibs) != 2 {
		t.Fatalf("abis = %v libs = %+v", info.ABIs, info.NativeLibs)
	}

	if l := info.NativeLibs[0]; l.Machine != "EM_AARCH64" || l.LoadAlign != 16384 || !l.Aligned16K || !l.Stored {
		t.Errorf("lib = %+v", l)
	}

	if !info.PageSize16K || !info.ZipChecks.Aligned4 || !info.ZipChecks.ArscStored {
		t.Errorf("zip checks = %+v page16k=%v", info.ZipChecks, info.PageSize16K)
	}
}

func checkDeps(t *testing.T, info *Info) {
	t.Helper()

	if info.BuildInfo.AGPVersion != "8.5.0" || info.BuildInfo.DexFiles != 1 {
		t.Errorf("build info = %+v", info.BuildInfo)
	}

	deps := map[string]string{}
	for _, d := range info.Dependencies {
		deps[d.Group+":"+d.Artifact] = d.Version
	}

	if deps["androidx.core:core"] != "1.13.1" || deps["com.squareup.okhttp3:okhttp"] != "4.12.0" {
		t.Errorf("dependencies = %v", deps)
	}
}

func checkManifest(t *testing.T, info *Info) {
	t.Helper()

	if len(info.Manifest.Permissions) != 2 || len(info.Manifest.Components) != 1 || !info.Manifest.Components[0].Launcher {
		t.Errorf("manifest = %+v", info.Manifest)
	}

	if !strings.Contains(info.ManifestXML, `android:versionName="1.2.3"`) {
		t.Errorf("manifest xml:\n%s", info.ManifestXML)
	}
}

// A re-signed copy has the same content digest; a modified one does not.
func TestContentDigestIgnoresSignature(t *testing.T) {
	b := sampleBuilder()
	a := inspectBytes(t, b.Build(apktest.Sign{Key: apktest.NewRSAKey("a"), V1: true, V2: true}))
	c := inspectBytes(t, b.Build(apktest.Sign{Key: apktest.NewECKey("b"), V2: true, V3: true}))

	if a.ContentDigest != c.ContentDigest {
		t.Errorf("re-signed digest differs: %s vs %s", a.ContentDigest, c.ContentDigest)
	}

	if a.SignerSHA256() == c.SignerSHA256() {
		t.Error("signers should differ")
	}

	b.Files["assets/readme.txt"] = []byte("patched")
	d := inspectBytes(t, b.Build(apktest.Sign{Key: apktest.NewECKey("b"), V2: true}))

	if d.ContentDigest == a.ContentDigest {
		t.Error("modified content should change the digest")
	}
}

func TestTamperedAPKDoesNotVerify(t *testing.T) {
	key := apktest.NewRSAKey("k")

	for _, sign := range []apktest.Sign{{Key: key, V2: true}, {Key: key, V1: true}} {
		raw := sampleBuilder().Build(sign)

		// Change one character of a title inside the stored resources.arsc
		// (v1 does not cover zip metadata, so the content itself is changed).
		i := bytes.Index(raw, []byte("E\x00s\x00e\x00m\x00"))
		if i < 0 {
			t.Fatal("title not found in archive")
		}

		raw[i] = 'X'

		info, err := Inspect(bytes.NewReader(raw), int64(len(raw)), Options{})
		if err != nil {
			continue // corrupting a header may make the zip unreadable: also a failure
		}

		if info.Signature.Verified {
			t.Errorf("%v: tampered APK verified", info.Signature.Schemes)
		}
	}
}

func TestInspectRejects(t *testing.T) {
	if _, err := Inspect(bytes.NewReader([]byte("not a zip")), 9, Options{}); !errors.Is(err, ErrNotZip) {
		t.Errorf("not a zip: %v", err)
	}

	b := &apktest.Builder{Package: "x"}
	raw := b.Build(apktest.Sign{})
	raw = bytes.ReplaceAll(raw, []byte("AndroidManifest.xml"), []byte("AndroidManifest.xmx"))

	if _, err := Inspect(bytes.NewReader(raw), int64(len(raw)), Options{}); !errors.Is(err, ErrNoManifest) {
		t.Errorf("no manifest: %v", err)
	}
}

func TestUnsignedWarns(t *testing.T) {
	info := inspectBytes(t, sampleBuilder().Build(apktest.Sign{}))
	if info.Signature.Verified || info.SignerSHA256() != "" || !slices.Contains(info.Warnings, "APK is not signed") {
		t.Errorf("signature = %+v warnings = %v", info.Signature, info.Warnings)
	}
}

func TestCertificateHelpers(t *testing.T) {
	key := apktest.NewRSAKey("Android Debug")

	ci, err := ParseCertificateFile(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: key.Cert}))
	if err != nil {
		t.Fatal(err)
	}

	if ci.SHA256 != key.SHA256() || !slices.Contains(ci.Flags, FlagDebugCert) || ci.PublicKeySize != 2048 {
		t.Errorf("cert = %+v", ci)
	}

	if der, err := ParseCertificateFile(key.Cert); err != nil || der.SHA256 != ci.SHA256 {
		t.Errorf("DER: %v", err)
	}

	pairs := make([]string, 0, 32)
	for i := 0; i < 64; i += 2 {
		pairs = append(pairs, strings.ToUpper(ci.SHA256[i:i+2]))
	}

	colon := strings.Join(pairs, ":")

	for _, in := range []string{ci.SHA256, colon, "SHA256: " + colon} {
		if got, ok := NormalizeFingerprint(in); !ok || got != ci.SHA256 {
			t.Errorf("NormalizeFingerprint(%q) = %q %v", in, got, ok)
		}
	}

	if _, ok := NormalizeFingerprint("abc"); ok {
		t.Error("short fingerprint accepted")
	}
}

// TestCorpus inspects real APKs from $OA_APK_CORPUS (not committed: third-party
// files). Every file must parse and verify.
func TestCorpus(t *testing.T) {
	dir := os.Getenv("OA_APK_CORPUS")
	if dir == "" {
		t.Skip("OA_APK_CORPUS not set")
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*.apk"))
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}

			info := inspectBytes(t, b)
			if !info.Signature.Verified || info.SignerSHA256() == "" || info.Manifest.Package == "" {
				t.Errorf("package %q signature %+v", info.Manifest.Package, info.Signature)
			}
		})
	}
}

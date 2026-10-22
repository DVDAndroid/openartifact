// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"path"
	"sort"
	"strings"
)

// Dependency is a library version recorded in the APK's META-INF.
type Dependency struct {
	Group    string `json:"group"`
	Artifact string `json:"artifact"`
	Version  string `json:"version"`
	Source   string `json:"source"`
}

// BuildInfo is what the build tools left in the APK.
type BuildInfo struct {
	AGPVersion  string `json:"agp_version,omitempty"`
	AppMetadata string `json:"app_metadata_version,omitempty"`
	Kotlin      bool   `json:"kotlin,omitempty"`
	Compose     bool   `json:"compose,omitempty"`
	DexFiles    int    `json:"dex_files"`
}

// frameworkMarkers maps entry names (or base names under lib/) to a
// cross-platform framework.
//
//nolint:gochecknoglobals // read-only lookup table
var frameworkMarkers = []struct {
	match     func(name, base string) bool
	framework string
}{
	{func(_, b string) bool { return b == "libflutter.so" }, "flutter"},
	{func(n, b string) bool {
		return b == "libreactnativejni.so" || b == "libreactnative.so" || n == "assets/index.android.bundle"
	}, "react-native"},
	{func(_, b string) bool { return b == "libhermes.so" }, "react-native"},
	{func(_, b string) bool { return b == "libunity.so" || b == "libil2cpp.so" }, "unity"},
	{func(n, b string) bool {
		return b == "libmonodroid.so" || b == "libmonosgen-2.0.so" || strings.HasPrefix(n, "assemblies/")
	}, "xamarin"},
	{func(n, _ string) bool { return n == "assets/www/cordova.js" }, "cordova"},
	{func(n, _ string) bool { return n == "assets/capacitor.config.json" }, "capacitor"},
	{func(_, b string) bool { return b == "libgodot_android.so" }, "godot"},
	{func(_, b string) bool { return b == "libgdx.so" }, "libgdx"},
	{func(_, b string) bool {
		return strings.HasPrefix(b, "libQt5Core") || strings.HasPrefix(b, "libQt6Core")
	}, "qt"},
	{func(_, b string) bool {
		return b == "libkivy.so" || b == "libpython3.so" || strings.HasPrefix(b, "libpython3.")
	}, "python"},
}

func (in *inspector) frameworksAndDeps() {
	fw := map[string]bool{}
	bi := &in.info.BuildInfo

	for _, n := range in.names {
		base := path.Base(n)

		for _, m := range frameworkMarkers {
			if m.match(n, base) {
				fw[m.framework] = true
			}
		}

		switch {
		case strings.HasPrefix(n, "classes") && strings.HasSuffix(n, ".dex") && !strings.Contains(n, "/"):
			bi.DexFiles++
		case strings.HasPrefix(n, "kotlin/") || strings.HasSuffix(n, ".kotlin_module"):
			bi.Kotlin = true
		}

		in.dependency(n)
	}

	if len(fw) == 0 && bi.DexFiles == 0 && len(in.info.NativeLibs) > 0 {
		fw["native"] = true
	}

	in.info.Frameworks = sortedKeys(fw)

	sort.Slice(in.info.Dependencies, func(i, j int) bool {
		a, b := in.info.Dependencies[i], in.info.Dependencies[j]
		if a.Group != b.Group {
			return a.Group < b.Group
		}

		return a.Artifact < b.Artifact
	})
}

// dependency records META-INF/<group>_<artifact>.version, Maven
// pom.properties and the AGP app-metadata file.
func (in *inspector) dependency(n string) {
	switch {
	case strings.HasPrefix(n, "META-INF/") && strings.HasSuffix(n, ".version") && strings.Count(n, "/") == 1:
		coord := strings.TrimSuffix(strings.TrimPrefix(n, "META-INF/"), ".version")

		g, a, ok := strings.Cut(coord, "_")
		if !ok {
			return
		}

		v := in.smallText(n)
		if v == "" || strings.ContainsAny(v, "\n\r") {
			return
		}

		in.info.Dependencies = append(in.info.Dependencies, Dependency{Group: g, Artifact: a, Version: v, Source: n})

		if g == "androidx.compose.runtime" {
			in.info.BuildInfo.Compose = true
		}
	case strings.HasPrefix(n, "META-INF/maven/") && strings.HasSuffix(n, "/pom.properties"):
		props := sectionAttrsEq(in.smallText(n))
		if props["artifactId"] != "" {
			in.info.Dependencies = append(in.info.Dependencies, Dependency{
				Group: props["groupId"], Artifact: props["artifactId"], Version: props["version"], Source: n,
			})
		}
	case n == "META-INF/com/android/build/gradle/app-metadata.properties":
		props := sectionAttrsEq(in.smallText(n))
		in.info.BuildInfo.AGPVersion = props["androidGradlePluginVersion"]
		in.info.BuildInfo.AppMetadata = props["appMetadataVersion"]
	}
}

func (in *inspector) smallText(n string) string {
	f := in.files[n]
	if f == nil {
		return ""
	}

	b, err := readEntry(f, 64<<10)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(b))
}

// sectionAttrsEq parses Java properties ("k=v" lines, # comments).
func sectionAttrsEq(s string) map[string]string {
	out := map[string]string{}

	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if k, v, ok := strings.Cut(line, "="); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	return out
}

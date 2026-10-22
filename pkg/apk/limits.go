// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import "errors"

// ErrLimit is returned (or recorded as a warning) when an input exceeds one of
// the parser limits.
var ErrLimit = errors.New("apk: limit exceeded")

// Limits bound the work Inspect does on untrusted input.
type Limits struct {
	MaxEntries      int   // zip entries
	MaxManifestSize int64 // uncompressed AndroidManifest.xml
	MaxArscSize     int64 // uncompressed resources.arsc
	MaxXMLSize      int64 // other binary XML (adaptive icons)
	MaxStrings      int   // strings per pool
	MaxStringLen    int   // characters per string
	MaxXMLNodes     int
	MaxXMLDepth     int
	MaxResTypes     int   // type chunks per package
	MaxRefDepth     int   // reference chain length
	MaxIcons        int   // icons extracted per file
	MaxIconSize     int64 // bytes per icon
	MaxELFSize      int64 // bytes of one compressed native library read for its headers
	MaxELFTotal     int64 // bytes of native libraries read in total
	MaxV1Size       int64 // bytes hashed for v1 (JAR) verification
	MaxMetaFileSize int64 // META-INF text files, signature files
}

// DefaultLimits returns limits suitable for large real-world APKs.
func DefaultLimits() Limits {
	return Limits{
		MaxEntries:      100_000,
		MaxManifestSize: 8 << 20,
		MaxArscSize:     128 << 20,
		MaxXMLSize:      1 << 20,
		MaxStrings:      2_000_000,
		MaxStringLen:    1 << 20,
		MaxXMLNodes:     200_000,
		MaxXMLDepth:     64,
		MaxResTypes:     20_000,
		MaxRefDepth:     8,
		MaxIcons:        64,
		MaxIconSize:     2 << 20,
		MaxELFSize:      256 << 20,
		MaxELFTotal:     1 << 30,
		MaxV1Size:       4 << 30,
		MaxMetaFileSize: 16 << 20,
	}
}

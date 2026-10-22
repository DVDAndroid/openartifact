// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apktest

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"slices"
	"sort"
)

// Framework attribute resource ids used by the builder.
const (
	AttrLabel       = 0x01010001
	AttrIcon        = 0x01010002
	AttrName        = 0x01010003
	AttrEnabled     = 0x0101000e
	AttrDebuggable  = 0x0101000f
	AttrExported    = 0x01010010
	AttrDrawable    = 0x01010199
	AttrMinSDK      = 0x0101020c
	AttrVersionCode = 0x0101021b
	AttrVersionName = 0x0101021c
	AttrTargetSDK   = 0x01010270
	AttrRoundIcon   = 0x0101052c
)

// Builder describes a synthetic APK.
type Builder struct {
	Package     string
	VersionCode int64
	VersionName string
	MinSDK      int
	TargetSDK   int
	Debuggable  bool
	Permissions []string
	// Labels maps a locale ("" default, "it", "pt-BR") to the app title.
	Labels map[string]string
	// Icons maps a density (160, 480, ...) to the size in pixels of a PNG
	// mipmap/ic_launcher generated for it.
	Icons map[uint16]int
	// Adaptive adds mipmap-anydpi-v26/ic_launcher.xml with a PNG foreground
	// and a color background.
	Adaptive bool
	// NativeLibs maps an ABI to library names; each gets a minimal ELF.
	NativeLibs map[string][]string
	// Files are extra entries (name -> content), deflated.
	Files map[string][]byte
}

// PNG returns a w×h opaque PNG.
func PNG(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 0x80, A: 0xff})
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)

	return buf.Bytes()
}

type entry struct {
	name   string
	data   []byte
	stored bool
	align  int
}

func (b *Builder) entries() []entry {
	tbl := &Table{Package: b.Package, Types: []string{"string", "mipmap", "drawable", "color"}, Entries: map[string][]ResEntry{}}

	// string/app_name
	labels := ResEntry{Key: "app_name"}
	for _, loc := range sortedKeys(b.Labels) {
		lang, region := splitLocale(loc)
		labels.Values = append(labels.Values, ResValue{Config: ResConfig{Language: lang, Region: region}, Type: TypeString, Str: b.Labels[loc]})
	}

	tbl.Entries["string"] = []ResEntry{labels}

	var out []entry

	icon := ResEntry{Key: "ic_launcher"}

	for _, d := range sortedDensities(b.Icons) {
		p := "res/mipmap-" + densityName(d) + "-v4/ic_launcher.png"
		icon.Values = append(icon.Values, ResValue{Config: ResConfig{Density: d}, Type: TypeString, Str: p})
		out = append(out, entry{name: p, data: PNG(b.Icons[d], b.Icons[d]), stored: true, align: 4})
	}

	if b.Adaptive {
		fg := "res/drawable-xxxhdpi-v4/ic_launcher_foreground.png"
		out = append(out, entry{name: fg, data: PNG(432, 432), stored: true, align: 4})
		tbl.Entries["drawable"] = []ResEntry{{Key: "ic_launcher_foreground", Values: []ResValue{
			{Config: ResConfig{Density: 640}, Type: TypeString, Str: fg},
		}}}
		tbl.Entries["color"] = []ResEntry{{Key: "ic_launcher_background", Values: []ResValue{
			{Type: TypeColor, Data: 0xff3ddc84},
		}}}

		xmlPath := "res/mipmap-anydpi-v26/ic_launcher.xml"
		icon.Values = append(icon.Values, ResValue{Config: ResConfig{Density: 0xfffe, SDK: 26}, Type: TypeString, Str: xmlPath})
		out = append(out, entry{name: xmlPath, data: EncodeXML(&XNode{Name: "adaptive-icon", Children: []*XNode{
			{Name: "background", Attrs: []XAttr{ARef("drawable", AttrDrawable, tbl.ID("color", 0))}},
			{Name: "foreground", Attrs: []XAttr{ARef("drawable", AttrDrawable, tbl.ID("drawable", 0))}},
		}})})
	}

	tbl.Entries["mipmap"] = []ResEntry{icon}

	out = append([]entry{
		{name: "AndroidManifest.xml", data: EncodeXML(b.manifest(tbl))},
		{name: "resources.arsc", data: tbl.Encode(), stored: true, align: 4},
		{name: "classes.dex", data: []byte("dex\n035\x00synthetic")},
	}, out...)

	for _, abi := range sortedKeys(b.NativeLibs) {
		for _, l := range b.NativeLibs[abi] {
			out = append(out, entry{name: "lib/" + abi + "/" + l, data: ELF(l), stored: true, align: 16384})
		}
	}

	for _, n := range sortedKeys(b.Files) {
		out = append(out, entry{name: n, data: b.Files[n]})
	}

	return out
}

func (b *Builder) manifest(tbl *Table) *XNode {
	app := &XNode{Name: "application", Attrs: []XAttr{ARef("label", AttrLabel, tbl.ID("string", 0))}}
	if len(b.Icons) > 0 || b.Adaptive {
		app.Attrs = append(app.Attrs, ARef("icon", AttrIcon, tbl.ID("mipmap", 0)))
	}

	if b.Debuggable {
		app.Attrs = append(app.Attrs, ABool("debuggable", AttrDebuggable, true))
	}

	app.Children = append(app.Children, &XNode{
		Name:  "activity",
		Attrs: []XAttr{AStr("name", AttrName, ".MainActivity"), ABool("exported", AttrExported, true)},
		Children: []*XNode{{Name: "intent-filter", Children: []*XNode{
			{Name: "action", Attrs: []XAttr{AStr("name", AttrName, "android.intent.action.MAIN")}},
			{Name: "category", Attrs: []XAttr{AStr("name", AttrName, "android.intent.category.LAUNCHER")}},
		}}},
	})

	root := &XNode{Name: "manifest", Attrs: []XAttr{
		AInt("versionCode", AttrVersionCode, b.VersionCode),
		AStr("versionName", AttrVersionName, b.VersionName),
		{Name: "package", Type: TypeString, Str: b.Package},
	}}

	root.Children = append(root.Children, &XNode{Name: "uses-sdk", Attrs: []XAttr{
		AInt("minSdkVersion", AttrMinSDK, int64(b.MinSDK)),
		AInt("targetSdkVersion", AttrTargetSDK, int64(b.TargetSDK)),
	}})

	for _, p := range b.Permissions {
		root.Children = append(root.Children, &XNode{Name: "uses-permission", Attrs: []XAttr{AStr("name", AttrName, p)}})
	}

	root.Children = append(root.Children, app)

	return root
}

// writeZip lays out entries (aligning stored ones) and returns the archive.
func writeZip(entries []entry) []byte {
	var buf bytes.Buffer

	zw := zip.NewWriter(&buf)

	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.name, Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(e.data), UncompressedSize64: uint64(len(e.data))}
		data := e.data

		if e.stored {
			fh.Method = zip.Store
			fh.CompressedSize64 = uint64(len(data))

			if e.align > 0 {
				fh.Extra = alignExtra(int64(buf.Len()), e.name, e.align)
			}
		} else {
			var c bytes.Buffer

			fw, _ := flate.NewWriter(&c, flate.BestCompression)
			_, _ = fw.Write(data)
			_ = fw.Close()
			data = c.Bytes()
			fh.CompressedSize64 = uint64(len(data))
		}

		w, err := zw.CreateRaw(fh)
		if err != nil {
			panic(err)
		}

		_, _ = w.Write(data)
		_ = zw.Flush()
	}

	_ = zw.Close()

	return buf.Bytes()
}

// alignExtra builds a zipalign-style (0xd935) extra field so the entry data
// starts at a multiple of align.
func alignExtra(offset int64, name string, align int) []byte {
	dataStart := offset + 30 + int64(len(name)) + 6
	pad := (int64(align) - dataStart%int64(align)) % int64(align)

	b := make([]byte, 6+pad)
	binary.LittleEndian.PutUint16(b, 0xd935)
	binary.LittleEndian.PutUint16(b[2:], uint16(2+pad)) //nolint:gosec // pad < align
	binary.LittleEndian.PutUint16(b[4:], uint16(align)) //nolint:gosec // small alignment

	return b
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

func sortedDensities(m map[uint16]int) []uint16 {
	out := make([]uint16, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}

func splitLocale(loc string) (string, string) {
	if len(loc) == 5 && loc[2] == '-' {
		return loc[:2], loc[3:]
	}

	return loc, ""
}

func densityName(d uint16) string {
	switch d {
	case 120:
		return "ldpi"
	case 160:
		return "mdpi"
	case 240:
		return "hdpi"
	case 320:
		return "xhdpi"
	case 480:
		return "xxhdpi"
	case 640:
		return "xxxhdpi"
	}

	return "nodpi"
}

// ELF returns a minimal AArch64 shared object header with one 16 KiB-aligned
// LOAD segment.
func ELF(_ string) []byte {
	b := make([]byte, 64+56)
	copy(b, "\x7fELF")
	b[4], b[5], b[6] = 2, 1, 1                 // 64-bit, little endian, version
	binary.LittleEndian.PutUint16(b[16:], 3)   // ET_DYN
	binary.LittleEndian.PutUint16(b[18:], 183) // EM_AARCH64
	binary.LittleEndian.PutUint32(b[20:], 1)   // version
	binary.LittleEndian.PutUint64(b[32:], 64)  // phoff
	binary.LittleEndian.PutUint16(b[52:], 64)  // ehsize
	binary.LittleEndian.PutUint16(b[54:], 56)  // phentsize
	binary.LittleEndian.PutUint16(b[56:], 1)   // phnum
	binary.LittleEndian.PutUint16(b[58:], 64)  // shentsize
	ph := b[64:]
	binary.LittleEndian.PutUint32(ph, 1)          // PT_LOAD
	binary.LittleEndian.PutUint32(ph[4:], 5)      // R+X
	binary.LittleEndian.PutUint64(ph[32:], 120)   // filesz
	binary.LittleEndian.PutUint64(ph[40:], 120)   // memsz
	binary.LittleEndian.PutUint64(ph[48:], 16384) // align

	return b
}

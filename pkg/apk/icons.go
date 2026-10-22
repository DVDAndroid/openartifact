// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

// Icon roles.
const (
	RoleApp        = "app"
	RoleRound      = "round"
	RoleForeground = "foreground"
	RoleBackground = "background"
	RoleMonochrome = "monochrome"
)

// Icon MIME types for non-raster icons.
const (
	MimeVectorXML = "application/vnd.android.vector+xml"
	MimeColor     = "application/vnd.android.color"
)

// Icon is one icon image (or adaptive icon layer) found in the APK.
type Icon struct {
	Role         string `json:"role"`
	Density      string `json:"density"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Mime         string `json:"mime"`
	SourcePath   string `json:"source_path"`
	ResourceName string `json:"resource_name,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	Size         int64  `json:"size"`
	Renderable   bool   `json:"renderable"`
	Color        string `json:"color,omitempty"`
	// Adaptive is true for the layers of an <adaptive-icon>.
	Adaptive bool   `json:"adaptive,omitempty"`
	Data     []byte `json:"-"`
}

type iconWalk struct {
	in   *inspector
	seen map[string]bool
}

func (in *inspector) icons() {
	m := &in.info.Manifest
	w := &iconWalk{in: in, seen: map[string]bool{}}

	icon := m.iconRef
	if !icon.Value.isRef() {
		icon = m.launcherIcon
	}

	if icon.Value.isRef() {
		in.info.IconResource = in.res.resourceName(icon.Value.Data)
		w.resource(RoleApp, icon.Value.Data, false, 0)
	}

	if m.roundIconRef.Value.isRef() {
		w.resource(RoleRound, m.roundIconRef.Value.Data, false, 0)
	}
}

// resource adds every config's file of resource id under role.
func (w *iconWalk) resource(role string, id uint32, adaptive bool, depth int) {
	if depth > 4 || w.in.res == nil {
		return
	}

	name := w.in.res.resourceName(id)

	for _, e := range w.in.res.resolveAll(id, 0) {
		v := e.value

		switch {
		case v.isString() && v.Str != "":
			w.file(role, v.Str, name, e.cfg, adaptive, depth)
		case v.Type >= typeColorFirst && v.Type <= typeColorLast:
			w.add(Icon{Role: role, Density: e.cfg.DensityName(), Mime: MimeColor, Color: fmt.Sprintf("#%08x", v.Data),
				ResourceName: name, Adaptive: adaptive, SourcePath: name})
		}
	}
}

func (w *iconWalk) file(role, p, resName string, cfg Config, adaptive bool, depth int) {
	f, ok := w.in.files[p]
	if !ok {
		w.in.warnf("icon %s: %s not in archive", resName, p)
		return
	}

	data, err := readEntry(f, w.in.opts.Limits.MaxIconSize)
	if err != nil {
		w.in.warnf("icon %s: %v", p, err)
		return
	}

	density := cfg.DensityName()
	if density == "" {
		density = densityFromPath(p)
	}

	ic := Icon{Role: role, Density: density, SourcePath: p, ResourceName: resName, Adaptive: adaptive, Size: int64(len(data))}

	if strings.HasSuffix(p, ".xml") {
		w.xmlDrawable(ic, data, depth)
		return
	}

	ic.Mime, ic.Width, ic.Height = imageInfo(data)
	if ic.Mime == "" {
		w.in.warnf("icon %s: unrecognised image format", p)
		return
	}

	ic.Renderable = true
	ic.Data = data
	w.add(ic)
}

// xmlDrawable follows adaptive icons, bitmaps and insets; vector drawables
// are kept as decoded XML (not rendered).
func (w *iconWalk) xmlDrawable(ic Icon, data []byte, depth int) {
	root, ns, err := parseAXML(data, w.in.opts.Limits)
	if err != nil {
		w.in.warnf("icon %s: %v", ic.SourcePath, err)
		return
	}

	switch root.Name {
	case "adaptive-icon":
		for _, c := range root.Children {
			role := map[string]string{"foreground": RoleForeground, "background": RoleBackground, "monochrome": RoleMonochrome}[c.Name]
			if role == "" {
				continue
			}

			w.layer(role, c, depth)
		}
	case "bitmap", "inset", "nine-patch":
		w.layer(ic.Role, root, depth)
	default:
		text := renderXML(root, ns)
		ic.Mime = MimeVectorXML
		ic.Data = []byte(text)
		ic.Size = int64(len(ic.Data))
		ic.Width, ic.Height = vectorSize(root)
		w.add(ic)
	}
}

// layer resolves the drawable of an adaptive icon layer (attribute or nested
// element such as <inset android:drawable=...>).
func (w *iconWalk) layer(role string, el *Element, depth int) {
	for _, attr := range []string{"drawable", "src"} {
		if a, ok := el.attr(attr); ok {
			switch {
			case a.Value.isRef():
				w.resource(role, a.Value.Data, true, depth+1)
			case a.Value.Type >= typeColorFirst && a.Value.Type <= typeColorLast:
				w.add(Icon{Role: role, Mime: MimeColor, Color: fmt.Sprintf("#%08x", a.Value.Data), Adaptive: true})
			}

			return
		}
	}

	for _, c := range el.Children {
		w.layer(role, c, depth+1)
	}
}

func (w *iconWalk) add(ic Icon) {
	key := ic.Role + "\x00" + ic.SourcePath + "\x00" + ic.Color
	if w.seen[key] {
		return
	}

	w.seen[key] = true

	if len(w.in.info.Icons) >= w.in.opts.Limits.MaxIcons {
		w.in.warnf("more than %d icons, the rest is skipped", w.in.opts.Limits.MaxIcons)
		return
	}

	if len(ic.Data) > 0 {
		sum := sha256.Sum256(ic.Data)
		ic.SHA256 = hex.EncodeToString(sum[:])
	}

	if w.in.opts.SkipIconData {
		ic.Data = nil
	}

	w.in.info.Icons = append(w.in.info.Icons, ic)
}

// densityFromPath reads the density qualifier of res/mipmap-xxhdpi-v4/x.png.
func densityFromPath(p string) string {
	dir := path.Base(path.Dir(p))
	for _, q := range strings.Split(dir, "-")[1:] {
		if strings.HasSuffix(q, "dpi") {
			return q
		}
	}

	return ""
}

func vectorSize(root *Element) (int, int) {
	return dimension(root, 0x01010159), dimension(root, 0x01010155) // android:width, android:height
}

// dimension reads a dp dimension attribute by resource id.
func dimension(e *Element, resID uint32) int {
	for _, a := range e.Attrs {
		if a.ResID == resID && a.Value.Type == typeDimension {
			// complex: mantissa in the top 24 bits, radix in bits 4-5.
			mant := int32(a.Value.Data&0xffffff00) >> 8 //nolint:gosec // signed mantissa
			radix := (a.Value.Data >> 4) & 3
			shift := []uint{0, 7, 15, 23}[radix]

			return int(mant) >> shift
		}
	}

	return 0
}

// imageInfo sniffs PNG, WebP and JPEG and returns MIME and pixel size.
func imageInfo(b []byte) (string, int, int) {
	switch {
	case len(b) >= 24 && bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", int(binary.BigEndian.Uint32(b[16:])), int(binary.BigEndian.Uint32(b[20:]))
	case len(b) >= 30 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		w, h := webpSize(b)
		return "image/webp", w, h
	case len(b) >= 4 && b[0] == 0xff && b[1] == 0xd8:
		w, h := jpegSize(b)
		return "image/jpeg", w, h
	}

	return "", 0, 0
}

func webpSize(b []byte) (int, int) {
	switch string(b[12:16]) {
	case "VP8X":
		w := int(b[24]) | int(b[25])<<8 | int(b[26])<<16
		h := int(b[27]) | int(b[28])<<8 | int(b[29])<<16

		return w + 1, h + 1
	case "VP8L":
		if len(b) < 25 {
			return 0, 0
		}

		v := binary.LittleEndian.Uint32(b[21:])

		return int(v&0x3fff) + 1, int((v>>14)&0x3fff) + 1
	case "VP8 ":
		return int(binary.LittleEndian.Uint16(b[26:]) & 0x3fff), int(binary.LittleEndian.Uint16(b[28:]) & 0x3fff)
	}

	return 0, 0
}

func jpegSize(b []byte) (int, int) {
	for i := 2; i+9 < len(b); {
		if b[i] != 0xff {
			return 0, 0
		}

		marker := b[i+1]
		size := int(binary.BigEndian.Uint16(b[i+2:]))

		if marker >= 0xc0 && marker <= 0xcf && marker != 0xc4 && marker != 0xc8 && marker != 0xcc {
			return int(binary.BigEndian.Uint16(b[i+7:])), int(binary.BigEndian.Uint16(b[i+5:]))
		}

		i += 2 + size
	}

	return 0, 0
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// Type chunk flags.
const (
	typeFlagSparse   = 0x01
	typeFlagOffset16 = 0x02

	entryFlagComplex = 0x0001
	entryFlagCompact = 0x0008

	appPackageID = 0x7f
)

// Config is the subset of ResTable_config that matters for titles and icons.
type Config struct {
	Language string `json:"language,omitempty"`
	Region   string `json:"region,omitempty"`
	Script   string `json:"script,omitempty"`
	Variant  string `json:"variant,omitempty"`
	Density  uint16 `json:"density,omitempty"`
	SDK      uint16 `json:"sdk,omitempty"`
	UIMode   uint8  `json:"ui_mode,omitempty"`
	// Other is true when the config carries qualifiers not decoded above
	// (screen size, orientation, ...), so a plainer config is preferred.
	Other bool `json:"other,omitempty"`
}

// Locale returns the BCP 47 tag of the config ("" for the default).
func (c Config) Locale() string {
	if c.Language == "" {
		return ""
	}

	parts := []string{c.Language}
	if c.Script != "" {
		parts = append(parts, c.Script)
	}

	if c.Region != "" {
		parts = append(parts, c.Region)
	}

	if c.Variant != "" {
		parts = append(parts, c.Variant)
	}

	return strings.Join(parts, "-")
}

// DensityName returns the aapt density qualifier (mdpi, xxhdpi, anydpi, ...).
func (c Config) DensityName() string {
	switch c.Density {
	case 0:
		return ""
	case 120:
		return "ldpi"
	case 160:
		return "mdpi"
	case 213:
		return "tvdpi"
	case 240:
		return "hdpi"
	case 320:
		return "xhdpi"
	case 480:
		return "xxhdpi"
	case 640:
		return "xxxhdpi"
	case 0xfffe:
		return "anydpi"
	case 0xffff:
		return "nodpi"
	}

	return fmt.Sprintf("%ddpi", c.Density)
}

// resEntry is one resolved (config, value) pair of a resource.
type resEntry struct {
	cfg   Config
	key   string
	value value
}

type resType struct {
	cfg     Config
	c       chunk
	flags   uint8
	count   int
	start   int // entriesStart, relative to chunk
	offsets int // offset table, relative to chunk
}

type resPackage struct {
	id    uint32
	name  string
	types map[uint8][]resType
	tpool *stringPool
	kpool *stringPool
}

// resTable is a decoded resources.arsc.
type resTable struct {
	strings  *stringPool
	packages map[uint32]*resPackage
	lim      Limits
}

func parseResTable(buf []byte, lim Limits) (*resTable, error) {
	root, err := readChunk(buf, 0)
	if err != nil {
		return nil, err
	}

	if root.typ != chunkTable {
		return nil, fmt.Errorf("%w: not a resource table", errTruncated)
	}

	t := &resTable{packages: map[uint32]*resPackage{}, lim: lim}

	err = root.children(func(c chunk) error {
		switch c.typ {
		case chunkStringPool:
			if t.strings == nil {
				p, err := parseStringPool(c, lim)
				if err != nil {
					return err
				}

				t.strings = p
			}
		case chunkTablePackage:
			p, err := parsePackage(c, lim)
			if err != nil {
				return err
			}

			if _, dup := t.packages[p.id]; !dup {
				t.packages[p.id] = p
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return t, nil
}

func parsePackage(c chunk, lim Limits) (*resPackage, error) {
	p := &resPackage{id: c.u32(8), types: map[uint8][]resType{}}

	if c.headerSize >= 12+256 {
		u := make([]uint16, 0, 128)
		for i := range 128 {
			ch := c.u16(12 + 2*i)
			if ch == 0 {
				break
			}

			u = append(u, ch)
		}

		p.name = string(utf16.Decode(u))
	}

	typeStrings, keyStrings := int(c.u32(268)), int(c.u32(276))
	types := 0

	err := c.children(func(ch chunk) error {
		switch ch.typ {
		case chunkStringPool:
			sp, err := parseStringPool(ch, lim)
			if err != nil {
				return err
			}

			switch {
			case p.tpool == nil && (typeStrings == 0 || sameOffset(c, ch, typeStrings)):
				p.tpool = sp
			case p.kpool == nil && (keyStrings == 0 || sameOffset(c, ch, keyStrings)):
				p.kpool = sp
			}
		case chunkTableType:
			types++
			if types > lim.MaxResTypes {
				return fmt.Errorf("%w: too many resource type chunks", ErrLimit)
			}

			rt, ok := parseType(ch)
			if ok {
				id := ch.data[8]
				p.types[id] = append(p.types[id], rt)
			}
		}

		return nil
	})

	return p, err
}

// sameOffset reports whether child ch starts at offset off of parent c.
func sameOffset(parent, ch chunk, off int) bool {
	if off < 0 || off >= len(parent.data) {
		return false
	}

	return &parent.data[off] == &ch.data[0]
}

func parseType(c chunk) (resType, bool) {
	// ResTable_type: header(8) id(1) flags(1) reserved(2) entryCount(4) entriesStart(4) config.
	if c.headerSize < 20+4 || len(c.data) < c.headerSize {
		return resType{}, false
	}

	rt := resType{
		c:       c,
		flags:   c.data[9],
		count:   int(c.u32(12)),
		start:   int(c.u32(16)),
		offsets: c.headerSize,
		cfg:     parseConfig(c.data[20:c.headerSize]),
	}

	return rt, rt.start <= len(c.data)
}

func parseConfig(b []byte) Config {
	var cfg Config

	size := len(b)
	if size >= 4 {
		if s := int(binary.LittleEndian.Uint32(b)); s < size {
			size = s
		}
	}

	b = b[:size]
	if size < 28 {
		return cfg
	}

	cfg.Language = unpackLocale(b[8], b[9], 'a')
	cfg.Region = strings.ToUpper(unpackLocale(b[10], b[11], '0'))

	if binary.LittleEndian.Uint32(b[4:]) != 0 || b[12] != 0 || b[13] != 0 || binary.LittleEndian.Uint32(b[16:]) != 0 ||
		binary.LittleEndian.Uint32(b[20:]) != 0 {
		cfg.Other = true // mcc/mnc, orientation, touchscreen, keyboard/navigation, screen size
	}

	cfg.Density = binary.LittleEndian.Uint16(b[14:])
	cfg.SDK = binary.LittleEndian.Uint16(b[24:])

	if size >= 32 {
		cfg.UIMode = b[29]
		if b[28] != 0 || binary.LittleEndian.Uint16(b[30:]) != 0 {
			cfg.Other = true
		}
	}

	if size >= 36 && binary.LittleEndian.Uint32(b[32:]) != 0 {
		cfg.Other = true
	}

	if size >= 48 {
		cfg.Script = strings.TrimRight(string(b[36:40]), "\x00")
		cfg.Variant = strings.TrimRight(string(b[40:48]), "\x00")
	}

	if cfg.UIMode != 0 {
		cfg.Other = true
	}

	return cfg
}

// unpackLocale decodes a 2-byte language or region code, including the
// packed 3-letter form.
func unpackLocale(in0, in1, base byte) string {
	if in0 == 0 && in1 == 0 {
		return ""
	}

	if in0&0x80 != 0 {
		first := in1 & 0x1f
		second := ((in1 & 0xe0) >> 5) + ((in0 & 0x03) << 3)
		third := (in0 & 0x7c) >> 2

		return string([]byte{first + base, second + base, third + base})
	}

	return string([]byte{in0, in1})
}

// entryAt returns the value of entry idx of rt.
func (t *resTable) entryAt(p *resPackage, rt resType, idx int) (resEntry, bool) {
	off, ok := rt.entryOffset(idx)
	if !ok {
		return resEntry{}, false
	}

	pos := rt.start + off
	c := rt.c

	if pos+8 > len(c.data) {
		return resEntry{}, false
	}

	size := int(c.u16(pos))
	flags := c.u16(pos + 2)

	if flags&entryFlagCompact != 0 {
		v := value{Type: uint8(flags >> 8), Data: c.u32(pos + 4)}
		return t.finish(p, rt, uint32(size), v), true //nolint:gosec // size is a uint16 read
	}

	key := c.u32(pos + 4)

	if flags&entryFlagComplex != 0 {
		return resEntry{cfg: rt.cfg, key: p.kpool.get(key)}, true
	}

	vpos := pos + size
	if size < 8 || vpos+8 > len(c.data) {
		return resEntry{}, false
	}

	v := value{Type: c.data[vpos+3], Data: c.u32(vpos + 4)}

	return t.finish(p, rt, key, v), true
}

func (t *resTable) finish(p *resPackage, rt resType, key uint32, v value) resEntry {
	if v.Type == typeString {
		v.Str = t.strings.get(v.Data)
	}

	return resEntry{cfg: rt.cfg, key: p.kpool.get(key), value: v}
}

func (rt resType) entryOffset(idx int) (int, bool) {
	c := rt.c

	switch {
	case rt.flags&typeFlagSparse != 0:
		// Sorted (u16 index, u16 offset/4) pairs.
		lo, hi := 0, rt.count
		for lo < hi {
			mid := (lo + hi) / 2
			pos := rt.offsets + mid*4
			if pos+4 > len(c.data) {
				return 0, false
			}

			switch e := int(c.u16(pos)); {
			case e == idx:
				return int(c.u16(pos+2)) * 4, true
			case e < idx:
				lo = mid + 1
			default:
				hi = mid
			}
		}

		return 0, false
	case idx < 0 || idx >= rt.count:
		return 0, false
	case rt.flags&typeFlagOffset16 != 0:
		o := c.u16(rt.offsets + idx*2)
		if o == 0xffff {
			return 0, false
		}

		return int(o) * 4, true
	default:
		o := c.u32(rt.offsets + idx*4)
		if o == noIndex || rt.offsets+idx*4+4 > len(c.data) {
			return 0, false
		}

		return int(o), true
	}
}

// lookup returns every config's value of resource id (no reference following).
func (t *resTable) lookup(id uint32) []resEntry {
	if t == nil {
		return nil
	}

	pkgID := id >> 24
	if pkgID == 0 {
		pkgID = appPackageID // dynamic reference to the app's own package
	}

	p := t.packages[pkgID]
	if p == nil {
		return nil
	}

	typeID := uint8(id >> 16) //nolint:gosec // byte of the resource id
	idx := int(id & 0xffff)

	var out []resEntry

	for _, rt := range p.types[typeID] {
		if e, ok := t.entryAt(p, rt, idx); ok {
			out = append(out, e)
		}
	}

	return out
}

// typeName returns the resource type name of id (string, mipmap, ...).
func (t *resTable) typeName(id uint32) string {
	if t == nil {
		return ""
	}

	pkgID := id >> 24
	if pkgID == 0 {
		pkgID = appPackageID
	}

	p := t.packages[pkgID]
	if p == nil {
		return ""
	}

	return p.tpool.get((id>>16)&0xff - 1)
}

// resourceName returns "type/key" for id, when known.
func (t *resTable) resourceName(id uint32) string {
	entries := t.lookup(id)
	if len(entries) == 0 {
		return ""
	}

	return t.typeName(id) + "/" + entries[0].key
}

// resolveAll expands id into its per-config values, following references
// (depth-limited). Values that are references to framework resources are
// returned unresolved.
func (t *resTable) resolveAll(id uint32, depth int) []resEntry {
	if t == nil || depth > t.lim.MaxRefDepth {
		return nil
	}

	var out []resEntry

	for _, e := range t.lookup(id) {
		if e.value.isRef() && e.value.Data != 0 && e.value.Data != id && (e.value.Data>>24 == appPackageID || e.value.Data>>24 == 0) {
			for _, sub := range t.resolveAll(e.value.Data, depth+1) {
				sub.cfg = mergeConfig(e.cfg, sub.cfg)
				out = append(out, sub)
			}

			continue
		}

		out = append(out, e)
	}

	return out
}

// mergeConfig keeps the more specific qualifiers of a referring and a
// referred config (a values-it string pointing at a default string is "it").
func mergeConfig(outer, inner Config) Config {
	if inner.Language == "" {
		inner.Language, inner.Region, inner.Script, inner.Variant = outer.Language, outer.Region, outer.Script, outer.Variant
	}

	if inner.Density == 0 {
		inner.Density = outer.Density
	}

	if inner.SDK < outer.SDK {
		inner.SDK = outer.SDK
	}

	inner.Other = inner.Other || outer.Other

	return inner
}

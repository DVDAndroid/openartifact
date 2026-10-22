// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apktest

import (
	"bytes"
	"encoding/binary"
	"sort"
	"unicode/utf16"
)

// ResConfig is the qualifier set of one resource value.
type ResConfig struct {
	Language string // "it"
	Region   string // "BR"
	Density  uint16 // 160, 480, 0xfffe (anydpi)
	SDK      uint16
}

// ResValue is one (config, value) of a resource entry.
type ResValue struct {
	Config ResConfig
	Type   uint8
	Data   uint32
	Str    string // for TypeString
}

// ResEntry is one named resource of a type.
type ResEntry struct {
	Key    string
	Values []ResValue
}

// Table is a resource table with one package (0x7f) whose types are
// numbered from 1 in the given order.
type Table struct {
	Package string
	Types   []string              // type names: "string", "mipmap", ...
	Entries map[string][]ResEntry // per type name, entry index = position
}

// ID returns the resource id of entry idx of type name.
func (t *Table) ID(typeName string, idx int) uint32 {
	for i, n := range t.Types {
		if n == typeName {
			return 0x7f000000 | uint32(i+1)<<16 | uint32(idx) //nolint:gosec // test data
		}
	}

	panic("unknown type " + typeName)
}

func encodeConfig(c ResConfig) []byte {
	b := make([]byte, 64)
	binary.LittleEndian.PutUint32(b, 64)
	copy(b[8:10], c.Language)
	copy(b[10:12], c.Region)
	binary.LittleEndian.PutUint16(b[14:], c.Density)
	binary.LittleEndian.PutUint16(b[24:], c.SDK)

	return b
}

// Encode writes resources.arsc.
func (t *Table) Encode() []byte {
	var values, typeNames, keys strPool

	for _, n := range t.Types {
		typeNames.add(n)
	}

	var body bytes.Buffer

	for ti, typeName := range t.Types {
		entries := t.Entries[typeName]

		// Group values by config: one type chunk per config.
		configs := map[ResConfig]map[int]ResValue{}

		for ei, e := range entries {
			keys.add(e.Key)

			for _, v := range e.Values {
				if configs[v.Config] == nil {
					configs[v.Config] = map[int]ResValue{}
				}

				configs[v.Config][ei] = v
			}
		}

		cfgs := make([]ResConfig, 0, len(configs))
		for c := range configs {
			cfgs = append(cfgs, c)
		}

		sort.Slice(cfgs, func(i, j int) bool { return cfgKey(cfgs[i]) < cfgKey(cfgs[j]) })

		for _, c := range cfgs {
			body.Write(typeChunk(ti+1, entries, configs[c], c, &values, &keys))
		}
	}

	tp, kp := typeNames.encode(), keys.encode()
	pkgHdr := 288

	var pkg bytes.Buffer

	pkg.Write(chunkHdr(0x0200, pkgHdr, pkgHdr+len(tp)+len(kp)+body.Len()))
	_ = binary.Write(&pkg, binary.LittleEndian, uint32(0x7f))

	name := make([]uint16, 128)
	copy(name, utf16.Encode([]rune(t.Package)))
	_ = binary.Write(&pkg, binary.LittleEndian, name)
	_ = binary.Write(&pkg, binary.LittleEndian, []uint32{
		uint32(pkgHdr), uint32(len(t.Types)), uint32(pkgHdr + len(tp)), 0, 0, //nolint:gosec // test data
	})
	pkg.Write(tp)
	pkg.Write(kp)
	pkg.Write(body.Bytes())

	vp := values.encode()

	var out bytes.Buffer

	out.Write(chunkHdr(0x0002, 12, 12+len(vp)+pkg.Len()))
	_ = binary.Write(&out, binary.LittleEndian, uint32(1))
	out.Write(vp)
	out.Write(pkg.Bytes())

	return out.Bytes()
}

func cfgKey(c ResConfig) string {
	return c.Language + "-" + c.Region + "-" + string(rune(c.Density)) + "-" + string(rune(c.SDK))
}

func typeChunk(typeID int, entries []ResEntry, vals map[int]ResValue, c ResConfig, values, keys *strPool) []byte {
	cfg := encodeConfig(c)
	hdr := 20 + len(cfg)
	count := len(entries)

	var data bytes.Buffer

	offsets := make([]uint32, count)

	for i := range entries {
		v, ok := vals[i]
		if !ok {
			offsets[i] = 0xffffffff
			continue
		}

		offsets[i] = uint32(data.Len()) //nolint:gosec // test data

		d := v.Data
		if v.Type == TypeString {
			d = values.add(v.Str)
		}

		_ = binary.Write(&data, binary.LittleEndian, []uint16{8, 0})
		_ = binary.Write(&data, binary.LittleEndian, keys.add(entries[i].Key))
		_ = binary.Write(&data, binary.LittleEndian, uint16(8))
		data.WriteByte(0)
		data.WriteByte(v.Type)
		_ = binary.Write(&data, binary.LittleEndian, d)
	}

	start := hdr + 4*count

	var out bytes.Buffer

	out.Write(chunkHdr(0x0201, hdr, start+data.Len()))
	out.WriteByte(byte(typeID)) //nolint:gosec // few types in tests
	out.WriteByte(0)
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, []uint32{uint32(count), uint32(start)}) //nolint:gosec // test data
	out.Write(cfg)
	_ = binary.Write(&out, binary.LittleEndian, offsets)
	out.Write(data.Bytes())

	return out.Bytes()
}

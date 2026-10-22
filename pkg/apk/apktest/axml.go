// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apktest builds small synthetic APKs (binary manifest, resource
// table, icons, native libraries, v1/v2/v3 signatures) for tests.
package apktest

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

const androidNS = "http://schemas.android.com/apk/res/android"

// Value types used by the encoder.
const (
	TypeReference = 0x01
	TypeString    = 0x03
	TypeIntDec    = 0x10
	TypeBool      = 0x12
	TypeColor     = 0x1c
)

// XAttr is one attribute of an encoded element. ResID 0 means no resource
// id; NS "" means no namespace, otherwise android:.
type XAttr struct {
	Android bool
	Name    string
	ResID   uint32
	Type    uint8
	Data    uint32
	Str     string
}

// XNode is one element of an encoded binary XML document.
type XNode struct {
	Name     string
	Attrs    []XAttr
	Children []*XNode
}

// AStr is an android: string attribute.
func AStr(name string, id uint32, s string) XAttr {
	return XAttr{Android: true, Name: name, ResID: id, Type: TypeString, Str: s}
}

// AInt is an android: integer attribute.
func AInt(name string, id uint32, n int64) XAttr {
	return XAttr{Android: true, Name: name, ResID: id, Type: TypeIntDec, Data: uint32(n)} //nolint:gosec // test data
}

// ABool is an android: boolean attribute.
func ABool(name string, id uint32, b bool) XAttr {
	var d uint32
	if b {
		d = 0xffffffff
	}

	return XAttr{Android: true, Name: name, ResID: id, Type: TypeBool, Data: d}
}

// ARef is an android: reference attribute (@0x7f...).
func ARef(name string, id, ref uint32) XAttr {
	return XAttr{Android: true, Name: name, ResID: id, Type: TypeReference, Data: ref}
}

type strPool struct {
	list  []string
	index map[string]uint32
}

func (p *strPool) add(s string) uint32 {
	if p.index == nil {
		p.index = map[string]uint32{}
	}

	if i, ok := p.index[s]; ok {
		return i
	}

	i := uint32(len(p.list)) //nolint:gosec // test data
	p.list = append(p.list, s)
	p.index[s] = i

	return i
}

// encode writes a UTF-16 ResStringPool chunk.
func (p *strPool) encode() []byte {
	var data bytes.Buffer

	offsets := make([]uint32, len(p.list))

	for i, s := range p.list {
		offsets[i] = uint32(data.Len()) //nolint:gosec // test data
		u := utf16.Encode([]rune(s))
		_ = binary.Write(&data, binary.LittleEndian, uint16(len(u))) //nolint:gosec // short test strings
		_ = binary.Write(&data, binary.LittleEndian, u)
		_ = binary.Write(&data, binary.LittleEndian, uint16(0))
	}

	for data.Len()%4 != 0 {
		data.WriteByte(0)
	}

	hdr := 28
	start := hdr + 4*len(p.list)

	var out bytes.Buffer

	w32 := func(v int) { _ = binary.Write(&out, binary.LittleEndian, uint32(v)) } //nolint:gosec // test data
	_ = binary.Write(&out, binary.LittleEndian, uint16(0x0001))
	_ = binary.Write(&out, binary.LittleEndian, uint16(hdr))
	w32(start + data.Len())
	w32(len(p.list))
	w32(0)
	w32(0)
	w32(start)
	w32(0)

	for _, o := range offsets {
		_ = binary.Write(&out, binary.LittleEndian, o)
	}

	out.Write(data.Bytes())

	return out.Bytes()
}

func chunkHdr(typ uint16, hdrSize, size int) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint16(b, typ)
	binary.LittleEndian.PutUint16(b[2:], uint16(hdrSize)) //nolint:gosec // test data
	binary.LittleEndian.PutUint32(b[4:], uint32(size))    //nolint:gosec // test data

	return b
}

// EncodeXML encodes root as a binary XML document (AXML).
func EncodeXML(root *XNode) []byte {
	var pool strPool

	// Attribute names with resource ids come first so their indices line up
	// with the resource map.
	var resIDs []uint32

	walk(root, func(n *XNode) {
		for _, a := range n.Attrs {
			if a.ResID != 0 {
				if _, ok := pool.index[a.Name]; !ok {
					pool.add(a.Name)
					resIDs = append(resIDs, a.ResID)
				}
			}
		}
	})

	nsPrefix, nsURI := pool.add("android"), pool.add(androidNS)

	var body bytes.Buffer

	writeNS := func(typ uint16) {
		body.Write(chunkHdr(typ, 16, 24))
		_ = binary.Write(&body, binary.LittleEndian, []uint32{1, 0xffffffff, nsPrefix, nsURI})
	}

	writeNS(0x0100)
	encodeNode(&body, &pool, root, nsURI)
	writeNS(0x0101)

	sp := pool.encode()

	rm := chunkHdr(0x0180, 8, 8+4*len(resIDs))
	for _, id := range resIDs {
		rm = binary.LittleEndian.AppendUint32(rm, id)
	}

	var out bytes.Buffer

	out.Write(chunkHdr(0x0003, 8, 8+len(sp)+len(rm)+body.Len()))
	out.Write(sp)
	out.Write(rm)
	out.Write(body.Bytes())

	return out.Bytes()
}

func walk(n *XNode, fn func(*XNode)) {
	fn(n)

	for _, c := range n.Children {
		walk(c, fn)
	}
}

func encodeNode(body *bytes.Buffer, pool *strPool, n *XNode, nsURI uint32) {
	name := pool.add(n.Name)
	size := 16 + 20 + 20*len(n.Attrs)

	body.Write(chunkHdr(0x0102, 16, size))
	_ = binary.Write(body, binary.LittleEndian, []uint32{1, 0xffffffff, 0xffffffff, name})
	_ = binary.Write(body, binary.LittleEndian, []uint16{20, 20, uint16(len(n.Attrs)), 0, 0, 0}) //nolint:gosec // test data

	for _, a := range n.Attrs {
		ns := uint32(0xffffffff)
		if a.Android {
			ns = nsURI
		}

		raw, data := uint32(0xffffffff), a.Data
		if a.Type == TypeString {
			raw = pool.add(a.Str)
			data = raw
		}

		_ = binary.Write(body, binary.LittleEndian, []uint32{ns, pool.add(a.Name), raw})
		_ = binary.Write(body, binary.LittleEndian, uint16(8))
		body.WriteByte(0)
		body.WriteByte(a.Type)
		_ = binary.Write(body, binary.LittleEndian, data)
	}

	for _, c := range n.Children {
		encodeNode(body, pool, c, nsURI)
	}

	body.Write(chunkHdr(0x0103, 16, 24))
	_ = binary.Write(body, binary.LittleEndian, []uint32{1, 0xffffffff, 0xffffffff, name})
}

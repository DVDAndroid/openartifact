// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
)

// Resource chunk types (frameworks/base/libs/androidfw/include/androidfw/ResourceTypes.h).
const (
	chunkStringPool     = 0x0001
	chunkTable          = 0x0002
	chunkXML            = 0x0003
	chunkXMLStartNS     = 0x0100
	chunkXMLEndNS       = 0x0101
	chunkXMLStartElem   = 0x0102
	chunkXMLEndElem     = 0x0103
	chunkXMLCData       = 0x0104
	chunkXMLResourceMap = 0x0180
	chunkTablePackage   = 0x0200
	chunkTableType      = 0x0201
	chunkTableTypeSpec  = 0x0202
	chunkTableLibrary   = 0x0203

	chunkHeaderSize = 8
	noIndex         = 0xffffffff
)

var errTruncated = errors.New("truncated chunk")

// chunk is one ResChunk_header-framed region of a resource file.
type chunk struct {
	typ        uint16
	headerSize int
	data       []byte // the whole chunk, header included
}

// readChunk reads the chunk starting at buf[off:].
func readChunk(buf []byte, off int) (chunk, error) {
	if off < 0 || off+chunkHeaderSize > len(buf) {
		return chunk{}, errTruncated
	}

	typ := binary.LittleEndian.Uint16(buf[off:])
	hs := int(binary.LittleEndian.Uint16(buf[off+2:]))
	size := int(binary.LittleEndian.Uint32(buf[off+4:]))

	if hs < chunkHeaderSize || size < hs || off+size > len(buf) || size < 0 {
		return chunk{}, fmt.Errorf("%w: type 0x%04x at %d (header %d, size %d)", errTruncated, typ, off, hs, size)
	}

	return chunk{typ: typ, headerSize: hs, data: buf[off : off+size]}, nil
}

// children iterates over the chunks following c's header.
func (c chunk) children(fn func(chunk) error) error {
	for off := c.headerSize; off < len(c.data); {
		ch, err := readChunk(c.data, off)
		if err != nil {
			return err
		}

		if err := fn(ch); err != nil {
			return err
		}

		off += len(ch.data)
	}

	return nil
}

func (c chunk) u16(off int) uint16 {
	if off+2 > len(c.data) {
		return 0
	}

	return binary.LittleEndian.Uint16(c.data[off:])
}

func (c chunk) u32(off int) uint32 {
	if off+4 > len(c.data) {
		return 0
	}

	return binary.LittleEndian.Uint32(c.data[off:])
}

// stringPool is a decoded ResStringPool. Strings are decoded lazily and cached.
type stringPool struct {
	c       chunk
	count   int
	utf8    bool
	start   int
	cache   []*string
	maxSize int
}

const stringPoolUTF8 = 1 << 8

func parseStringPool(c chunk, lim Limits) (*stringPool, error) {
	if c.typ != chunkStringPool || c.headerSize < 28 {
		return nil, fmt.Errorf("%w: not a string pool", errTruncated)
	}

	count := int(c.u32(8))
	flags := c.u32(16)
	start := int(c.u32(20))

	if count < 0 || count > lim.MaxStrings || c.headerSize+count*4 > len(c.data) {
		return nil, fmt.Errorf("%w: string pool with %d strings", ErrLimit, count)
	}

	return &stringPool{
		c:       c,
		count:   count,
		utf8:    flags&stringPoolUTF8 != 0,
		start:   start,
		cache:   make([]*string, count),
		maxSize: lim.MaxStringLen,
	}, nil
}

// get returns string i, or "" when i is out of range or malformed.
func (p *stringPool) get(i uint32) string {
	if p == nil || i == noIndex || int(i) >= p.count {
		return ""
	}

	if s := p.cache[i]; s != nil {
		return *s
	}

	off := p.start + int(p.c.u32(p.c.headerSize+int(i)*4))

	var s string
	if p.utf8 {
		s = p.decodeUTF8(off)
	} else {
		s = p.decodeUTF16(off)
	}

	p.cache[i] = &s

	return s
}

func (p *stringPool) decodeUTF8(off int) string {
	b := p.c.data
	if off < 0 || off >= len(b) {
		return ""
	}

	// UTF-16 length (skipped), then UTF-8 byte length; each 1 or 2 bytes.
	_, n := utf8Len(b, off)
	off += n

	length, n := utf8Len(b, off)
	off += n

	if length > p.maxSize || off+length > len(b) {
		return ""
	}

	return string(b[off : off+length])
}

func utf8Len(b []byte, off int) (int, int) {
	if off >= len(b) {
		return 0, 1
	}

	if b[off]&0x80 == 0 {
		return int(b[off]), 1
	}

	if off+1 >= len(b) {
		return 0, 2
	}

	return int(b[off]&0x7f)<<8 | int(b[off+1]), 2
}

func (p *stringPool) decodeUTF16(off int) string {
	b := p.c.data
	if off < 0 || off+2 > len(b) {
		return ""
	}

	length := int(binary.LittleEndian.Uint16(b[off:]))
	off += 2

	if length&0x8000 != 0 {
		if off+2 > len(b) {
			return ""
		}

		length = (length&0x7fff)<<16 | int(binary.LittleEndian.Uint16(b[off:]))
		off += 2
	}

	if length > p.maxSize || off+length*2 > len(b) {
		return ""
	}

	u := make([]uint16, length)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[off+2*i:])
	}

	return string(utf16.Decode(u))
}

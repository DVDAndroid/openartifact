// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"html"
	"strings"
)

// AndroidNS is the android: attribute namespace.
const AndroidNS = "http://schemas.android.com/apk/res/android"

var errNotAXML = errors.New("not a binary XML document")

// androidAttrs maps framework attribute resource IDs to their names. The
// manifest is looked up by ID, as the platform does, so stripped or
// obfuscated attribute names (AndResGuard, packers) still resolve.
//
//nolint:gochecknoglobals // read-only lookup table
var androidAttrs = map[uint32]string{
	0x01010000: "theme",
	0x01010001: "label",
	0x01010002: "icon",
	0x01010003: "name",
	0x01010006: "permission",
	0x01010007: "readPermission",
	0x01010008: "writePermission",
	0x01010009: "protectionLevel",
	0x0101000b: "sharedUserId",
	0x0101000c: "hasCode",
	0x0101000e: "enabled",
	0x0101000f: "debuggable",
	0x01010010: "exported",
	0x01010011: "process",
	0x01010018: "authorities",
	0x0101001c: "priority",
	0x01010024: "value",
	0x01010025: "resource",
	0x01010026: "mimeType",
	0x01010027: "scheme",
	0x01010028: "host",
	0x01010029: "port",
	0x0101002a: "path",
	0x0101002b: "pathPrefix",
	0x0101002c: "pathPattern",
	0x01010119: "src",
	0x01010199: "drawable",
	0x0101020c: "minSdkVersion",
	0x0101021b: "versionCode",
	0x0101021c: "versionName",
	0x01010270: "targetSdkVersion",
	0x01010271: "maxSdkVersion",
	0x01010272: "testOnly",
	0x01010280: "allowBackup",
	0x01010281: "glEsVersion",
	0x0101028e: "required",
	0x010102b7: "installLocation",
	0x010104ea: "extractNativeLibs",
	0x010104ec: "usesCleartextTraffic",
	0x01010527: "networkSecurityConfig",
	0x0101052c: "roundIcon",
	0x01010572: "compileSdkVersion",
	0x01010573: "compileSdkVersionCodename",
	0x01010576: "versionCodeMajor",
}

// Attr is one decoded attribute of a binary XML element.
type Attr struct {
	NS    string
	Name  string
	ResID uint32
	Raw   string
	Value value
}

// Element is one decoded binary XML element.
type Element struct {
	NS       string
	Name     string
	Attrs    []Attr
	Children []*Element
}

// attr returns the android: attribute called name.
func (e *Element) attr(name string) (Attr, bool) {
	for _, a := range e.Attrs {
		if a.Name == name && (a.NS == AndroidNS || a.NS == "") {
			return a, true
		}
	}

	return Attr{}, false
}

// str returns the android: attribute as text ("" when absent).
func (e *Element) str(name string) string {
	a, ok := e.attr(name)
	if !ok {
		return ""
	}

	if a.Value.isString() || a.Raw != "" {
		if a.Raw != "" {
			return a.Raw
		}

		return a.Value.Str
	}

	return a.Value.String()
}

func (e *Element) intAttr(name string) (int64, bool) {
	a, ok := e.attr(name)
	if !ok {
		return 0, false
	}

	return a.Value.Int()
}

func (e *Element) boolAttr(name string) (bool, bool) {
	a, ok := e.attr(name)
	if !ok {
		return false, false
	}

	return a.Value.Bool()
}

func (e *Element) each(name string, fn func(*Element)) {
	for _, c := range e.Children {
		if c.Name == name {
			fn(c)
		}
	}
}

// axmlDecoder holds the state of one binary XML decode.
type axmlDecoder struct {
	pool   *stringPool
	resMap []uint32
	nsURI  map[string]string // uri -> prefix, for rendering
	nsList []nsDecl
	lim    Limits
}

type nsDecl struct{ prefix, uri string }

// parseAXML decodes a binary XML document into an element tree.
func parseAXML(buf []byte, lim Limits) (*Element, []nsDecl, error) {
	root, err := readChunk(buf, 0)
	if err != nil || root.typ != chunkXML {
		return nil, nil, errNotAXML
	}

	d := &axmlDecoder{nsURI: map[string]string{}, lim: lim}

	var (
		doc   *Element
		stack []*Element
		nodes int
	)

	err = root.children(func(c chunk) error {
		switch c.typ {
		case chunkStringPool:
			p, err := parseStringPool(c, lim)
			if err != nil {
				return err
			}

			d.pool = p
		case chunkXMLResourceMap:
			for off := c.headerSize; off+4 <= len(c.data); off += 4 {
				d.resMap = append(d.resMap, binary.LittleEndian.Uint32(c.data[off:]))
			}
		case chunkXMLStartNS:
			prefix, uri := d.pool.get(c.u32(16)), d.pool.get(c.u32(20))
			d.nsURI[uri] = prefix
			d.nsList = append(d.nsList, nsDecl{prefix, uri})
		case chunkXMLStartElem:
			nodes++
			if nodes > lim.MaxXMLNodes || len(stack) >= lim.MaxXMLDepth {
				return fmt.Errorf("%w: XML too large or too deep", ErrLimit)
			}

			el := d.startElement(c)
			if len(stack) == 0 {
				if doc == nil {
					doc = el
				}
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, el)
			}

			stack = append(stack, el)
		case chunkXMLEndElem:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	if doc == nil {
		return nil, nil, errNotAXML
	}

	return doc, d.nsList, nil
}

func (d *axmlDecoder) startElement(c chunk) *Element {
	// ResXMLTree_node (16 bytes) then ResXMLTree_attrExt.
	ext := c.headerSize
	el := &Element{NS: d.pool.get(c.u32(ext)), Name: d.pool.get(c.u32(ext + 4))}
	attrStart := int(c.u16(ext + 8))
	attrSize := int(c.u16(ext + 10))
	count := int(c.u16(ext + 12))

	if attrSize < 20 {
		attrSize = 20
	}

	for i := range count {
		off := ext + attrStart + i*attrSize
		if off+20 > len(c.data) {
			break
		}

		nameIdx := c.u32(off + 4)
		a := Attr{NS: d.pool.get(c.u32(off)), Name: d.pool.get(nameIdx)}

		if int(nameIdx) < len(d.resMap) {
			a.ResID = d.resMap[nameIdx]
			if n, ok := androidAttrs[a.ResID]; ok {
				a.Name = n
				if a.NS == "" {
					a.NS = AndroidNS
				}
			}
		}

		a.Raw = d.pool.get(c.u32(off + 8))
		a.Value = value{Type: c.data[off+15], Data: c.u32(off + 16)}

		if a.Value.Type == typeString {
			a.Value.Str = d.pool.get(a.Value.Data)
		}

		el.Attrs = append(el.Attrs, a)
	}

	return el
}

// renderXML renders the tree as indented XML text, for display only.
func renderXML(root *Element, ns []nsDecl) string {
	prefixes := map[string]string{}
	for _, n := range ns {
		prefixes[n.uri] = n.prefix
	}

	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	renderElement(&b, root, ns, prefixes, 0)

	return b.String()
}

func renderElement(b *strings.Builder, e *Element, ns []nsDecl, prefixes map[string]string, depth int) {
	indent := strings.Repeat("    ", depth)
	b.WriteString(indent + "<" + e.Name)

	if depth == 0 {
		for _, n := range ns {
			fmt.Fprintf(b, "\n%s    xmlns:%s=\"%s\"", indent, n.prefix, html.EscapeString(n.uri))
		}
	}

	for _, a := range e.Attrs {
		name := a.Name
		if name == "" {
			name = fmt.Sprintf("attr_0x%08x", a.ResID)
		}

		if p := prefixes[a.NS]; p != "" {
			name = p + ":" + name
		} else if a.NS == AndroidNS {
			name = "android:" + name
		}

		val := a.Raw
		if val == "" {
			val = a.Value.String()
		}

		fmt.Fprintf(b, "\n%s    %s=\"%s\"", indent, name, html.EscapeString(val))
	}

	if len(e.Children) == 0 {
		b.WriteString(" />\n")
		return
	}

	b.WriteString(">\n")

	for _, c := range e.Children {
		renderElement(b, c, ns, prefixes, depth+1)
	}

	b.WriteString(indent + "</" + e.Name + ">\n")
}

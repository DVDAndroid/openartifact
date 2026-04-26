// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func buildFullRPM() []byte {
	var buf bytes.Buffer

	lead := make([]byte, LeadSize)
	binary.BigEndian.PutUint32(lead[0:4], MagicNumber)
	lead[4] = 3                               // major version (uint8)
	lead[5] = 0                               // minor version (uint8)
	binary.BigEndian.PutUint16(lead[6:8], 0)  // type
	binary.BigEndian.PutUint16(lead[8:10], 1) // arch
	copy(lead[10:76], "test-pkg-1.0-1.el9.x86_64.rpm")
	binary.BigEndian.PutUint16(lead[76:78], 1) // os
	binary.BigEndian.PutUint16(lead[78:80], 5) // sigtype
	buf.Write(lead)

	sigData := []byte("sha256\x00abcdef1234\x00")
	sigEntries := []TagEntry{
		{Tag: 1000, Type: TagTypeSTRING, Offset: 0, Count: 1},
	}
	sigHeader := buildRawHeader(sigEntries, sigData)
	buf.Write(sigHeader)

	pad := sigHeaderTotalSize(sigEntries, sigData)

	padBytes := PadTo8(pad)
	if padBytes > 0 {
		buf.Write(make([]byte, padBytes))
	}

	pkgData := buildPkgData()
	pkgEntries := buildPkgEntries(pkgData)
	pkgHeader := buildRawHeader(pkgEntries, pkgData)
	buf.Write(pkgHeader)

	return buf.Bytes()
}

func buildPkgData() []byte {
	var data bytes.Buffer

	nameOff := 0

	data.WriteString("test-pkg")
	data.WriteByte(0)

	verOff := data.Len()
	data.WriteString("1.0")
	data.WriteByte(0)

	relOff := data.Len()
	data.WriteString("1.el9")
	data.WriteByte(0)

	summaryOff := data.Len()
	data.WriteString("A test package")
	data.WriteByte(0)

	_ = nameOff
	_ = verOff
	_ = relOff
	_ = summaryOff

	epochData := make([]byte, 4)
	binary.BigEndian.PutUint32(epochData, 2)
	data.Write(epochData)

	buildTimeData := make([]byte, 8)
	binary.BigEndian.PutUint64(buildTimeData, uint64(1700000000))
	data.Write(buildTimeData)

	sizeData := make([]byte, 4*3)
	binary.BigEndian.PutUint32(sizeData[0:4], 50000)
	binary.BigEndian.PutUint32(sizeData[4:8], 100000)
	binary.BigEndian.PutUint32(sizeData[8:12], 200000)
	data.Write(sizeData)

	archOff := data.Len()
	data.WriteString("x86_64")
	data.WriteByte(0)

	_ = archOff

	return data.Bytes()
}

func buildPkgEntries(data []byte) []TagEntry {
	off := int32(0)
	nameEnd := int32(bytes.IndexByte(data, 0)) + 1
	verEnd := nameEnd + int32(len("1.0")) + 1
	relEnd := verEnd + int32(len("1.el9")) + 1
	summaryEnd := relEnd + int32(len("A test package")) + 1
	epochOff := summaryEnd
	buildTimeOff := epochOff + 4
	sizeOff := buildTimeOff + 8
	archOff := sizeOff + 12

	return []TagEntry{
		{Tag: RPMTAG_NAME, Type: TagTypeSTRING, Offset: off, Count: 1},
		{Tag: RPMTAG_VERSION, Type: TagTypeSTRING, Offset: nameEnd, Count: 1},
		{Tag: RPMTAG_RELEASE, Type: TagTypeSTRING, Offset: verEnd, Count: 1},
		{Tag: RPMTAG_SUMMARY, Type: TagTypeSTRING, Offset: relEnd, Count: 1},
		{Tag: RPMTAG_EPOCH, Type: TagTypeINT32, Offset: epochOff, Count: 1},
		{Tag: RPMTAG_BUILDTIME, Type: TagTypeINT32, Offset: buildTimeOff, Count: 1},
		{Tag: RPMTAG_SIZE, Type: TagTypeINT32, Offset: sizeOff, Count: 3},
		{Tag: RPMTAG_ARCH, Type: TagTypeSTRING, Offset: archOff, Count: 1},
	}
}

func buildRawHeader(entries []TagEntry, data []byte) []byte {
	var buf bytes.Buffer

	intro := make([]byte, 16)
	intro[0] = 0x8E
	intro[1] = 0xAD
	intro[2] = 0xE8
	intro[3] = 1
	binary.BigEndian.PutUint32(intro[8:12], uint32(len(entries)))
	binary.BigEndian.PutUint32(intro[12:16], uint32(len(data)))
	buf.Write(intro)

	for _, e := range entries {
		entryBuf := make([]byte, 16)
		binary.BigEndian.PutUint32(entryBuf[0:4], uint32(e.Tag))
		binary.BigEndian.PutUint32(entryBuf[4:8], uint32(e.Type))
		binary.BigEndian.PutUint32(entryBuf[8:12], uint32(e.Offset))
		binary.BigEndian.PutUint32(entryBuf[12:16], uint32(e.Count))
		buf.Write(entryBuf)
	}

	buf.Write(data)

	return buf.Bytes()
}

func sigHeaderTotalSize(entries []TagEntry, data []byte) int64 {
	return int64(16) + int64(len(entries))*16 + int64(len(data))
}

func TestParseRPM_FullPackage(t *testing.T) {
	data := buildFullRPM()

	pkg, err := ParseRPM(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseRPM() error: %v", err)
	}

	if pkg.Name != "test-pkg" {
		t.Errorf("Name = %q, want %q", pkg.Name, "test-pkg")
	}

	if pkg.Version != "1.0" {
		t.Errorf("Version = %q, want %q", pkg.Version, "1.0")
	}

	if pkg.Arch != "x86_64" {
		t.Errorf("Arch = %q, want %q", pkg.Arch, "x86_64")
	}
}

func TestParseRPM_InvalidLead(t *testing.T) {
	buf := make([]byte, 96)
	binary.BigEndian.PutUint32(buf[0:4], 0xDEADBEEF)

	_, err := ParseRPM(bytes.NewReader(buf))
	if err == nil {
		t.Fatal("expected error for invalid lead magic")
	}
}

func TestPackage_GetMetadata(t *testing.T) {
	pkg := &Package{
		Name:          "test-pkg",
		Version:       "1.0",
		Release:       "1.el9",
		Epoch:         2,
		Arch:          "x86_64",
		Summary:       "A test package",
		PackageSize:   50000,
		InstalledSize: 100000,
		Provides:      []Dependency{{Name: "test-pkg", Flags: SENSE_EQUAL, Version: "1.0-1.el9"}},
	}

	meta := pkg.GetMetadata()
	if meta["name"] != "test-pkg" {
		t.Errorf("meta[name] = %v, want test-pkg", meta["name"])
	}

	if meta["epoch"] != uint32(2) {
		t.Errorf("meta[epoch] = %v, want 2", meta["epoch"])
	}

	if meta["format"] != "rpm" {
		t.Errorf("meta[format] = %v, want rpm", meta["format"])
	}

	if _, ok := meta["provides"]; !ok {
		t.Error("expected provides key in metadata")
	}
}

func TestParseRPM_ShortRead(t *testing.T) {
	_, err := ParseRPM(bytes.NewReader([]byte{0xED, 0xAB}))
	if err == nil {
		t.Fatal("expected error for short read")
	}
}

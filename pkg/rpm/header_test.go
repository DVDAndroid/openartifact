// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func buildHeader(name string, version string, epoch uint32) *Header {
	nameData := []byte(name)
	versionData := []byte(version)
	epochData := make([]byte, 4)
	binary.BigEndian.PutUint32(epochData, epoch)

	hsize := len(nameData) + 1 + len(versionData) + 1 + len(epochData)
	data := make([]byte, hsize)
	off := 0
	copy(data[off:], nameData)
	off += len(nameData) + 1
	copy(data[off:], versionData)
	off += len(versionData) + 1
	copy(data[off:], epochData)

	entries := []TagEntry{
		{Tag: RPMTAG_NAME, Type: TagTypeSTRING, Offset: 0, Count: 1},
		{Tag: RPMTAG_VERSION, Type: TagTypeSTRING, Offset: int32(len(nameData) + 1), Count: 1},
		{Tag: RPMTAG_EPOCH, Type: TagTypeINT32, Offset: int32(len(nameData) + 1 + len(versionData) + 1), Count: 1},
	}

	return &Header{
		Version: 1,
		NIndex:  int32(len(entries)),
		HSize:   int32(hsize),
		Entries: entries,
		Data:    data,
	}
}

func buildHeaderBytes(entries []TagEntry, data []byte) []byte {
	var buf bytes.Buffer

	intro := make([]byte, 16)
	intro[0] = 0x8E
	intro[1] = 0xAD
	intro[2] = 0xE8
	intro[3] = 1 // version
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

func TestParseHeader_Valid(t *testing.T) {
	nameData := append([]byte("test-pkg"), 0)
	entries := []TagEntry{
		{Tag: RPMTAG_NAME, Type: TagTypeSTRING, Offset: 0, Count: 1},
	}
	raw := buildHeaderBytes(entries, nameData)

	h, err := ParseHeader(bytes.NewReader(raw), 1<<24)
	if err != nil {
		t.Fatalf("ParseHeader() error: %v", err)
	}

	if h.NIndex != 1 {
		t.Errorf("NIndex = %d, want 1", h.NIndex)
	}

	if len(h.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(h.Entries))
	}

	if h.Entries[0].Tag != RPMTAG_NAME {
		t.Errorf("Entry tag = %d, want %d", h.Entries[0].Tag, RPMTAG_NAME)
	}

	name, ok := h.GetString(RPMTAG_NAME)
	if !ok {
		t.Fatal("GetString(RPMTAG_NAME) returned false")
	}

	if name != "test-pkg" {
		t.Errorf("Name = %q, want %q", name, "test-pkg")
	}
}

func TestParseHeader_InvalidMagic(t *testing.T) {
	intro := make([]byte, 16)
	intro[0] = 0xDE
	intro[1] = 0xAD
	intro[2] = 0xBE
	intro[3] = 1

	_, err := ParseHeader(bytes.NewReader(intro), 1<<24)
	if err == nil {
		t.Fatal("expected error for invalid header magic")
	}
}

func TestParseHeader_TooLarge(t *testing.T) {
	nameData := append([]byte("pkg"), 0)
	entries := []TagEntry{
		{Tag: RPMTAG_NAME, Type: TagTypeSTRING, Offset: 0, Count: 1},
	}
	raw := buildHeaderBytes(entries, nameData)

	_, err := ParseHeader(bytes.NewReader(raw), 10) // very small max
	if err == nil {
		t.Fatal("expected error for header exceeding max size")
	}
}

func TestHeader_GetString_Missing(t *testing.T) {
	h := buildHeader("pkg", "1.0", 0)

	_, ok := h.GetString(RPMTAG_ARCH) // tag not in header
	if ok {
		t.Error("expected GetString to return false for missing tag")
	}
}

func TestHeader_GetInt32s(t *testing.T) {
	epochData := make([]byte, 4)
	binary.BigEndian.PutUint32(epochData, 42)

	entries := []TagEntry{
		{Tag: RPMTAG_EPOCH, Type: TagTypeINT32, Offset: 0, Count: 1},
	}
	raw := buildHeaderBytes(entries, epochData)

	h, err := ParseHeader(bytes.NewReader(raw), 1<<24)
	if err != nil {
		t.Fatalf("ParseHeader() error: %v", err)
	}

	vals, ok := h.GetInt32s(RPMTAG_EPOCH)
	if !ok {
		t.Fatal("GetInt32s(RPMTAG_EPOCH) returned false")
	}

	if len(vals) != 1 || vals[0] != 42 {
		t.Errorf("vals = %v, want [42]", vals)
	}
}

func TestHeader_GetUint32(t *testing.T) {
	epochData := make([]byte, 4)
	binary.BigEndian.PutUint32(epochData, 7)

	entries := []TagEntry{
		{Tag: RPMTAG_EPOCH, Type: TagTypeINT32, Offset: 0, Count: 1},
	}
	raw := buildHeaderBytes(entries, epochData)

	h, err := ParseHeader(bytes.NewReader(raw), 1<<24)
	if err != nil {
		t.Fatalf("ParseHeader() error: %v", err)
	}

	val, ok := h.GetUint32(RPMTAG_EPOCH)
	if !ok {
		t.Fatal("GetUint32(RPMTAG_EPOCH) returned false")
	}

	if val != 7 {
		t.Errorf("val = %d, want 7", val)
	}
}

func TestHeader_GetStringArray(t *testing.T) {
	data := []byte("libfoo\x00libbar\x00")
	entries := []TagEntry{
		{Tag: RPMTAG_REQUIRENAME, Type: TagTypeSTRING_ARRAY, Offset: 0, Count: 2},
	}
	raw := buildHeaderBytes(entries, data)

	h, err := ParseHeader(bytes.NewReader(raw), 1<<24)
	if err != nil {
		t.Fatalf("ParseHeader() error: %v", err)
	}

	names, ok := h.GetStringArray(RPMTAG_REQUIRENAME)
	if !ok {
		t.Fatal("GetStringArray(RPMTAG_REQUIRENAME) returned false")
	}

	if len(names) != 2 {
		t.Fatalf("len(names) = %d, want 2", len(names))
	}

	if names[0] != "libfoo" || names[1] != "libbar" {
		t.Errorf("names = %v, want [libfoo, libbar]", names)
	}
}

func TestHeader_TotalSize(t *testing.T) {
	h := &Header{NIndex: 3, HSize: 100}

	expected := int64(16) + int64(3)*16 + int64(100)
	if got := h.TotalSize(); got != expected {
		t.Errorf("TotalSize() = %d, want %d", got, expected)
	}
}

func TestPadTo8(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{8, 0},
		{16, 0},
		{9, 7},
		{10, 6},
		{1, 7},
		{0, 0},
	}
	for _, tt := range tests {
		got := PadTo8(tt.input)
		if got != tt.expected {
			t.Errorf("PadTo8(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

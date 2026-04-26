// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestParseARHeader(t *testing.T) {
	// Create a valid AR header
	headerBuf := make([]byte, 60)
	copy(headerBuf[0:16], "test.txt        ")
	copy(headerBuf[16:28], "1234567890  ")
	copy(headerBuf[28:34], "0     ")
	copy(headerBuf[34:40], "0     ")
	copy(headerBuf[40:48], "100644  ")
	copy(headerBuf[48:58], "1024      ")
	headerBuf[58] = '`'
	headerBuf[59] = '\n'

	header, err := parseARHeader(headerBuf)
	if err != nil {
		t.Fatalf("parseARHeader() error = %v", err)
	}

	if header.Name != "test.txt" {
		t.Errorf("Name = %v, want test.txt", header.Name)
	}

	if header.Size != 1024 {
		t.Errorf("Size = %v, want 1024", header.Size)
	}
}

func TestParseARHeaderInvalid(t *testing.T) {
	tests := []struct {
		name    string
		setup   func([]byte)
		wantErr bool
	}{
		{
			name: "invalid ending marker",
			setup: func(buf []byte) {
				buf[58] = 'X'
				buf[59] = 'Y'
			},
			wantErr: true,
		},
		{
			name: "invalid size",
			setup: func(buf []byte) {
				copy(buf[48:58], "invalid   ")
				buf[58] = '`'
				buf[59] = '\n'
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headerBuf := make([]byte, 60)
			copy(headerBuf[0:16], "test.txt        ")
			copy(headerBuf[16:28], "1234567890  ")
			copy(headerBuf[28:34], "0     ")
			copy(headerBuf[34:40], "0     ")
			copy(headerBuf[40:48], "100644  ")
			copy(headerBuf[48:58], "1024      ")
			headerBuf[58] = '`'
			headerBuf[59] = '\n'

			tt.setup(headerBuf)

			_, err := parseARHeader(headerBuf)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseARHeader() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadARArchive(t *testing.T) {
	// Create a minimal AR archive
	var buf bytes.Buffer

	// Write magic
	buf.WriteString("!<arch>\n")

	// Write a member
	memberName := "test.txt"
	memberData := []byte("Hello, World!")
	memberSize := int64(len(memberData))

	// Create header
	header := make([]byte, 60)
	copy(header[0:16], fmt.Sprintf("%-16s", memberName))
	copy(header[16:28], fmt.Sprintf("%-12d", time.Now().Unix()))
	copy(header[28:34], "0     ")
	copy(header[34:40], "0     ")
	copy(header[40:48], "100644  ")
	copy(header[48:58], fmt.Sprintf("%-10d", memberSize))
	header[58] = '`'
	header[59] = '\n'

	buf.Write(header)
	buf.Write(memberData)

	// Read archive
	archive, err := ReadARArchive(&buf, 0)
	if err != nil {
		t.Fatalf("ReadARArchive() error = %v", err)
	}

	if len(archive.Members) != 1 {
		t.Errorf("Members count = %d, want 1", len(archive.Members))
	}

	if archive.Members[0].Header.Name != memberName {
		t.Errorf("Member name = %v, want %v", archive.Members[0].Header.Name, memberName)
	}

	if !bytes.Equal(archive.Members[0].Data, memberData) {
		t.Errorf("Member data = %v, want %v", archive.Members[0].Data, memberData)
	}
}

func TestARArchiveGetMember(t *testing.T) {
	archive := &ARArchive{
		Members: []*ARMember{
			{
				Header: ARHeader{Name: "file1.txt"},
				Data:   []byte("data1"),
			},
			{
				Header: ARHeader{Name: "file2.txt"},
				Data:   []byte("data2"),
			},
		},
	}

	member := archive.GetMember("file1.txt")
	if member == nil {
		t.Fatalf("GetMember() returned nil")
	}

	if !bytes.Equal(member.Data, []byte("data1")) {
		t.Errorf("GetMember() data = %v, want data1", member.Data)
	}

	member = archive.GetMember("nonexistent.txt")
	if member != nil {
		t.Errorf("GetMember() returned non-nil for nonexistent member")
	}
}

func TestARArchiveGetMemberData(t *testing.T) {
	archive := &ARArchive{
		Members: []*ARMember{
			{
				Header: ARHeader{Name: "test.txt"},
				Data:   []byte("test data"),
			},
		},
	}

	data := archive.GetMemberData("test.txt")
	if !bytes.Equal(data, []byte("test data")) {
		t.Errorf("GetMemberData() = %v, want test data", data)
	}

	data = archive.GetMemberData("nonexistent.txt")
	if data != nil {
		t.Errorf("GetMemberData() returned non-nil for nonexistent member")
	}
}

func TestARArchiveListMembers(t *testing.T) {
	archive := &ARArchive{
		Members: []*ARMember{
			{Header: ARHeader{Name: "file1.txt"}},
			{Header: ARHeader{Name: "file2.txt"}},
			{Header: ARHeader{Name: "file3.txt"}},
		},
	}

	members := archive.ListMembers()
	if len(members) != 3 {
		t.Errorf("ListMembers() returned %d members, want 3", len(members))
	}

	expected := []string{"file1.txt", "file2.txt", "file3.txt"}
	for i, name := range members {
		if name != expected[i] {
			t.Errorf("ListMembers()[%d] = %v, want %v", i, name, expected[i])
		}
	}
}

func TestARArchiveGetMemberReader(t *testing.T) {
	archive := &ARArchive{
		Members: []*ARMember{
			{
				Header: ARHeader{Name: "test.txt"},
				Data:   []byte("test data"),
			},
		},
	}

	reader := archive.GetMemberReader("test.txt")
	if reader == nil {
		t.Errorf("GetMemberReader() returned nil")
	}

	data := make([]byte, 9)

	n, err := reader.Read(data)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}

	if n != 9 {
		t.Errorf("Read() returned %d bytes, want 9", n)
	}

	if !bytes.Equal(data, []byte("test data")) {
		t.Errorf("Read() data = %v, want test data", data)
	}
}

func buildARMemberHeader(name string, sizeStr string) []byte {
	header := make([]byte, 60)
	copy(header[0:16], fmt.Sprintf("%-16s", name))
	copy(header[16:28], fmt.Sprintf("%-12d", time.Now().Unix()))
	copy(header[28:34], "0     ")
	copy(header[34:40], "0     ")
	copy(header[40:48], "100644  ")
	copy(header[48:58], fmt.Sprintf("%-10s", sizeStr))
	header[58] = '`'
	header[59] = '\n'

	return header
}

func TestReadARArchiveRejectsNegativeSize(t *testing.T) {
	var buf bytes.Buffer

	buf.WriteString("!<arch>\n")
	buf.Write(buildARMemberHeader("evil.txt", "-1"))

	_, err := ReadARArchive(&buf, 0)
	if err == nil {
		t.Fatal("ReadARArchive() expected error for negative member size, got nil")
	}
}

func TestReadARArchiveRejectsOversizedMember(t *testing.T) {
	var buf bytes.Buffer

	buf.WriteString("!<arch>\n")
	buf.Write(buildARMemberHeader("bomb.txt", "999999999"))

	// maxMemberSize smaller than the declared size must be rejected before
	// any large allocation is attempted; the reader has far fewer bytes
	// than declared, so an unbounded implementation would still try to
	// allocate ~1GB for the read buffer.
	_, err := ReadARArchive(&buf, 1024)
	if err == nil {
		t.Fatal("ReadARArchive() expected error for oversized member declaration, got nil")
	}

	if !errors.Is(err, ErrARMemberTooLarge) {
		t.Errorf("ReadARArchive() error = %v, want wrapping ErrARMemberTooLarge", err)
	}
}

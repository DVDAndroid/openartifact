// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func buildValidLead() []byte {
	buf := make([]byte, LeadSize)
	binary.BigEndian.PutUint32(buf[0:4], MagicNumber)
	buf[4] = 3                               // major version (uint8)
	buf[5] = 0                               // minor version (uint8)
	binary.BigEndian.PutUint16(buf[6:8], 0)  // type
	binary.BigEndian.PutUint16(buf[8:10], 1) // arch
	copy(buf[10:76], "test-package-1.0-1.x86_64.rpm")
	binary.BigEndian.PutUint16(buf[76:78], 1) // os
	binary.BigEndian.PutUint16(buf[78:80], 5) // sigtype

	return buf
}

func TestParseLead_Valid(t *testing.T) {
	data := buildValidLead()

	lead, err := ParseLead(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseLead() error: %v", err)
	}

	if lead.MajorVersion != 3 {
		t.Errorf("MajorVersion = %d, want 3", lead.MajorVersion)
	}

	if lead.MinorVersion != 0 {
		t.Errorf("MinorVersion = %d, want 0", lead.MinorVersion)
	}

	if lead.Arch != 1 {
		t.Errorf("Arch = %d, want 1", lead.Arch)
	}

	if lead.SignatureType != 5 {
		t.Errorf("SignatureType = %d, want 5", lead.SignatureType)
	}

	if lead.Name != "test-package-1.0-1.x86_64.rpm" {
		t.Errorf("Name = %q, want %q", lead.Name, "test-package-1.0-1.x86_64.rpm")
	}
}

func TestParseLead_InvalidMagic(t *testing.T) {
	buf := make([]byte, LeadSize)
	binary.BigEndian.PutUint32(buf[0:4], 0xDEADBEEF)

	_, err := ParseLead(bytes.NewReader(buf))
	if err == nil {
		t.Fatal("expected error for invalid magic number")
	}
}

func TestParseLead_VersionTooLow(t *testing.T) {
	buf := make([]byte, LeadSize)
	binary.BigEndian.PutUint32(buf[0:4], MagicNumber)
	buf[4] = 2 // major version 2, below minimum of 3

	_, err := ParseLead(bytes.NewReader(buf))
	if err == nil {
		t.Fatal("expected error for version < 3")
	}
}

func TestParseLead_ShortRead(t *testing.T) {
	_, err := ParseLead(bytes.NewReader([]byte{0xED, 0xAB, 0xEE}))
	if err == nil {
		t.Fatal("expected error for short read")
	}
}

func TestParseLead_Version4(t *testing.T) {
	buf := make([]byte, LeadSize)
	binary.BigEndian.PutUint32(buf[0:4], MagicNumber)
	buf[4] = 4 // major version (uint8)

	lead, err := ParseLead(bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("ParseLead() error: %v", err)
	}

	if lead.MajorVersion != 4 {
		t.Errorf("MajorVersion = %d, want 4", lead.MajorVersion)
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Sentinel errors for RPM lead parsing.
var (
	ErrInvalidLeadMagic   = errors.New("rpm: lead magic")
	ErrUnsupportedVersion = errors.New("rpm: lead version")
)

const (
	MagicNumber uint32 = 0xEDABEEDB
	LeadSize    int    = 96
)

type Lead struct {
	MajorVersion  uint8
	MinorVersion  uint8
	Type          uint16
	Arch          uint16
	Name          string
	OS            uint16
	SignatureType uint16
}

func ParseLead(r io.Reader) (*Lead, error) {
	var buf [LeadSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return nil, fmt.Errorf("rpm: lead read: %w", err)
	}

	magic := binary.BigEndian.Uint32(buf[0:4])
	if magic != MagicNumber {
		return nil, fmt.Errorf("%w: got %#x, want %#x", ErrInvalidLeadMagic, magic, MagicNumber)
	}

	major := buf[4]
	if major < 3 {
		return nil, fmt.Errorf("%w: unsupported major %d (minimum 3)", ErrUnsupportedVersion, major)
	}

	return &Lead{
		MajorVersion:  major,
		MinorVersion:  buf[5],
		Type:          binary.BigEndian.Uint16(buf[6:8]),
		Arch:          binary.BigEndian.Uint16(buf[8:10]),
		Name:          strings.TrimRight(string(buf[10:76]), "\x00"),
		OS:            binary.BigEndian.Uint16(buf[76:78]),
		SignatureType: binary.BigEndian.Uint16(buf[78:80]),
	}, nil
}

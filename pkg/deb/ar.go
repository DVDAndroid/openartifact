// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package deb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Sentinel errors for AR archive parsing.
var (
	ErrInvalidARMagic     = errors.New("invalid AR archive magic")
	ErrInvalidHeaderSize  = errors.New("invalid header size")
	ErrInvalidModTime     = errors.New("invalid modification time")
	ErrInvalidOwnerID     = errors.New("invalid owner ID")
	ErrInvalidGroupID     = errors.New("invalid group ID")
	ErrInvalidMode        = errors.New("invalid mode")
	ErrInvalidSize        = errors.New("invalid size")
	ErrARMemberTooLarge   = errors.New("AR member size exceeds maximum allowed")
	ErrInvalidARHeaderEnd = errors.New("invalid AR header ending marker")
)

// ARHeader represents the header of an AR archive member.
type ARHeader struct {
	Name    string
	ModTime time.Time
	OwnerID int
	GroupID int
	Mode    int
	Size    int64
}

// ARMember represents a member file in an AR archive.
type ARMember struct {
	Header ARHeader
	Data   []byte
}

// ARArchive represents a Debian .deb file (which is an AR archive).
type ARArchive struct {
	Members []*ARMember
}

const (
	arMagic       = "!<arch>\n"
	arHeaderSize  = 60
	arMemberAlign = 2

	// maxARMemberSize is the default sanity cap on a single AR member's
	// declared payload size, used when the caller does not pass a tighter
	// bound. It guards against a corrupt or malicious header driving an
	// unbounded allocation.
	maxARMemberSize = 1 << 30 // 1 GiB
)

// ReadARArchive reads and parses an AR archive. maxMemberSize bounds the
// declared size of any single member (guarding against a corrupt or
// malicious header causing an unbounded or negative allocation); pass <= 0
// to fall back to maxARMemberSize. Callers that already hold the full
// input in memory should pass its length so a member can never be
// declared larger than the remaining stream.
func ReadARArchive(r io.Reader, maxMemberSize int64) (*ARArchive, error) {
	if maxMemberSize <= 0 {
		maxMemberSize = maxARMemberSize
	}
	// Read and verify magic number
	magic := make([]byte, len(arMagic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("failed to read AR magic: %w", err)
	}

	if string(magic) != arMagic {
		return nil, fmt.Errorf("%w: %s", ErrInvalidARMagic, string(magic))
	}

	archive := &ARArchive{
		Members: make([]*ARMember, 0),
	}

	for {
		// Read member header
		headerBuf := make([]byte, arHeaderSize)

		n, err := io.ReadFull(r, headerBuf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("failed to read AR member header: %w", err)
		}

		if n != arHeaderSize {
			break
		}

		// Parse header
		header, err := parseARHeader(headerBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to parse AR header: %w", err)
		}
		if header.Size < 0 {
			return nil, fmt.Errorf("%w: %d", ErrInvalidSize, header.Size)
		}

		if header.Size > maxMemberSize {
			return nil, fmt.Errorf("%w: member %q declares size %d, max %d",
				ErrARMemberTooLarge, header.Name, header.Size, maxMemberSize)
		}

		// Read member data
		data := make([]byte, header.Size)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("failed to read AR member data: %w", err)
		}

		member := &ARMember{
			Header: header,
			Data:   data,
		}
		archive.Members = append(archive.Members, member)

		// AR members are aligned to 2-byte boundaries
		if header.Size%arMemberAlign != 0 {
			padding := make([]byte, arMemberAlign-(header.Size%arMemberAlign))
			if _, err := io.ReadFull(r, padding); err != nil && err != io.EOF {
				return nil, fmt.Errorf("failed to read AR padding: %w", err)
			}
		}
	}

	return archive, nil
}

// parseARHeader parses a 60-byte AR member header.
func parseARHeader(buf []byte) (ARHeader, error) {
	if len(buf) != arHeaderSize {
		return ARHeader{}, fmt.Errorf("%w: %d", ErrInvalidHeaderSize, len(buf))
	}

	header := ARHeader{}

	// Name (16 bytes)
	name := strings.TrimRight(string(buf[0:16]), " ")
	// Handle extended filename format (name ends with /)
	name = strings.TrimSuffix(name, "/")
	header.Name = name

	// Modification time (12 bytes)
	modTimeStr := strings.TrimSpace(string(buf[16:28]))

	modTime, err := strconv.ParseInt(modTimeStr, 10, 64)
	if err != nil {
		return ARHeader{}, fmt.Errorf("%w: %s", ErrInvalidModTime, modTimeStr)
	}

	header.ModTime = time.Unix(modTime, 0)

	// Owner ID (6 bytes)
	ownerStr := strings.TrimSpace(string(buf[28:34]))

	owner, err := strconv.Atoi(ownerStr)
	if err != nil {
		return ARHeader{}, fmt.Errorf("%w: %s", ErrInvalidOwnerID, ownerStr)
	}

	header.OwnerID = owner

	// Group ID (6 bytes)
	groupStr := strings.TrimSpace(string(buf[34:40]))

	group, err := strconv.Atoi(groupStr)
	if err != nil {
		return ARHeader{}, fmt.Errorf("%w: %s", ErrInvalidGroupID, groupStr)
	}

	header.GroupID = group

	// Mode (8 bytes)
	modeStr := strings.TrimSpace(string(buf[40:48]))

	mode, err := strconv.ParseInt(modeStr, 8, 32)
	if err != nil {
		return ARHeader{}, fmt.Errorf("%w: %s", ErrInvalidMode, modeStr)
	}

	header.Mode = int(mode)

	// Size (10 bytes)
	sizeStr := strings.TrimSpace(string(buf[48:58]))

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return ARHeader{}, fmt.Errorf("%w: %s", ErrInvalidSize, sizeStr)
	}

	header.Size = size

	// Ending marker (2 bytes) - should be "`\n"
	if buf[58] != '`' || buf[59] != '\n' {
		return ARHeader{}, ErrInvalidARHeaderEnd
	}

	return header, nil
}

// GetMember retrieves a member by name.
func (a *ARArchive) GetMember(name string) *ARMember {
	for _, member := range a.Members {
		if member.Header.Name == name {
			return member
		}
	}

	return nil
}

// GetMemberData retrieves member data by name.
func (a *ARArchive) GetMemberData(name string) []byte {
	member := a.GetMember(name)
	if member == nil {
		return nil
	}

	return member.Data
}

// GetMemberReader returns an io.Reader for a member.
func (a *ARArchive) GetMemberReader(name string) io.Reader {
	data := a.GetMemberData(name)
	if data == nil {
		return nil
	}

	return bytes.NewReader(data)
}

// ListMembers returns a list of member names.
func (a *ARArchive) ListMembers() []string {
	names := make([]string, len(a.Members))
	for i, member := range a.Members {
		names[i] = member.Header.Name
	}

	return names
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build integration

package integration_test

import (
	"bytes"
	"encoding/binary"

	pkgrpm "github.com/openartifact/openartifact/pkg/rpm"
)

func CreateTestRPM(name, version, release, arch string) []byte {
	var buf bytes.Buffer

	lead := make([]byte, pkgrpm.LeadSize)
	binary.BigEndian.PutUint32(lead[0:4], pkgrpm.MagicNumber)
	lead[4] = 3
	lead[5] = 0
	copy(lead[10:76], []byte(name+"-"+version+"-"+release+"."+arch+".rpm"))
	buf.Write(lead)

	sigData := []byte("\x00")
	sigEntries := []pkgrpm.TagEntry{
		{Tag: 1000, Type: pkgrpm.TagTypeSTRING, Offset: 0, Count: 1},
	}
	sigHeader := buildHeaderBytes(sigEntries, sigData)
	buf.Write(sigHeader)

	sigTotal := int64(16) + int64(len(sigEntries))*16 + int64(len(sigData))
	if pad := pkgrpm.PadTo8(sigTotal); pad > 0 {
		buf.Write(make([]byte, pad))
	}

	nameBytes := []byte(name + "\x00")
	verBytes := []byte(version + "\x00")
	relBytes := []byte(release + "\x00")
	archBytes := []byte(arch + "\x00")
	summaryBytes := []byte("Test package " + name + "\x00")
	descBytes := []byte("Integration test package\x00")
	licenseBytes := []byte("MIT\x00")
	groupBytes := []byte("Development/Tools\x00")
	osBytes := []byte("linux\x00")
	payloadFmtBytes := []byte("cpio\x00")
	payloadCmpBytes := []byte("gzip\x00")

	var pkgData bytes.Buffer
	offsets := make(map[int32]int32)
	cur := int32(0)

	offsets[pkgrpm.RPMTAG_NAME] = cur
	pkgData.Write(nameBytes)
	cur += int32(len(nameBytes))

	offsets[pkgrpm.RPMTAG_VERSION] = cur
	pkgData.Write(verBytes)
	cur += int32(len(verBytes))

	offsets[pkgrpm.RPMTAG_RELEASE] = cur
	pkgData.Write(relBytes)
	cur += int32(len(relBytes))

	offsets[pkgrpm.RPMTAG_ARCH] = cur
	pkgData.Write(archBytes)
	cur += int32(len(archBytes))

	offsets[pkgrpm.RPMTAG_SUMMARY] = cur
	pkgData.Write(summaryBytes)
	cur += int32(len(summaryBytes))

	offsets[pkgrpm.RPMTAG_DESCRIPTION] = cur
	pkgData.Write(descBytes)
	cur += int32(len(descBytes))

	offsets[pkgrpm.RPMTAG_LICENSE] = cur
	pkgData.Write(licenseBytes)
	cur += int32(len(licenseBytes))

	offsets[pkgrpm.RPMTAG_GROUP] = cur
	pkgData.Write(groupBytes)
	cur += int32(len(groupBytes))

	offsets[1021] = cur
	pkgData.Write(osBytes)
	cur += int32(len(osBytes))

	offsets[1124] = cur
	pkgData.Write(payloadFmtBytes)
	cur += int32(len(payloadFmtBytes))

	offsets[1125] = cur
	pkgData.Write(payloadCmpBytes)
	cur += int32(len(payloadCmpBytes))

	_ = cur

	pkgEntries := []pkgrpm.TagEntry{
		{Tag: pkgrpm.RPMTAG_NAME, Type: pkgrpm.TagTypeSTRING, Offset: offsets[pkgrpm.RPMTAG_NAME], Count: 1},
		{Tag: pkgrpm.RPMTAG_VERSION, Type: pkgrpm.TagTypeSTRING, Offset: offsets[pkgrpm.RPMTAG_VERSION], Count: 1},
		{Tag: pkgrpm.RPMTAG_RELEASE, Type: pkgrpm.TagTypeSTRING, Offset: offsets[pkgrpm.RPMTAG_RELEASE], Count: 1},
		{Tag: pkgrpm.RPMTAG_ARCH, Type: pkgrpm.TagTypeSTRING, Offset: offsets[pkgrpm.RPMTAG_ARCH], Count: 1},
		{Tag: pkgrpm.RPMTAG_SUMMARY, Type: pkgrpm.TagTypeI18NSTRING, Offset: offsets[pkgrpm.RPMTAG_SUMMARY], Count: 1},
		{Tag: pkgrpm.RPMTAG_DESCRIPTION, Type: pkgrpm.TagTypeI18NSTRING, Offset: offsets[pkgrpm.RPMTAG_DESCRIPTION], Count: 1},
		{Tag: pkgrpm.RPMTAG_LICENSE, Type: pkgrpm.TagTypeSTRING, Offset: offsets[pkgrpm.RPMTAG_LICENSE], Count: 1},
		{Tag: pkgrpm.RPMTAG_GROUP, Type: pkgrpm.TagTypeI18NSTRING, Offset: offsets[pkgrpm.RPMTAG_GROUP], Count: 1},
		{Tag: 1021, Type: pkgrpm.TagTypeSTRING, Offset: offsets[1021], Count: 1},
		{Tag: 1124, Type: pkgrpm.TagTypeSTRING, Offset: offsets[1124], Count: 1},
		{Tag: 1125, Type: pkgrpm.TagTypeSTRING, Offset: offsets[1125], Count: 1},
	}

	pkgHeader := buildHeaderBytes(pkgEntries, pkgData.Bytes())
	buf.Write(pkgHeader)

	return buf.Bytes()
}

func buildHeaderBytes(entries []pkgrpm.TagEntry, data []byte) []byte {
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
		eb := make([]byte, 16)
		binary.BigEndian.PutUint32(eb[0:4], uint32(e.Tag))
		binary.BigEndian.PutUint32(eb[4:8], uint32(e.Type))
		binary.BigEndian.PutUint32(eb[8:12], uint32(e.Offset))
		binary.BigEndian.PutUint32(eb[12:16], uint32(e.Count))
		buf.Write(eb)
	}
	buf.Write(data)
	return buf.Bytes()
}

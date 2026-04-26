// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"fmt"
	"io"
)

type FileEntry struct {
	Path   string
	Size   int64
	Mode   uint16
	MTime  int64
	Digest string
	LinkTo string
	IsDir  bool
}

type ChangelogEntry struct {
	Time        int64
	Author      string
	Description string
}

type Package struct {
	Name          string
	Version       string
	Release       string
	Epoch         uint32
	Arch          string
	Summary       string
	Description   string
	License       string
	URL           string
	Vendor        string
	Group         string
	Packager      string
	SourceRPM     string
	BuildHost     string
	BuildTime     int64
	PackageSize   int64
	InstalledSize int64
	ArchiveSize   int64
	SHA256        string
	HeaderStart   int64
	HeaderEnd     int64
	Provides      []Dependency
	Requires      []Dependency
	Conflicts     []Dependency
	Obsoletes     []Dependency
	Suggests      []Dependency
	Supplements   []Dependency
	Enhances      []Dependency
	Files         []FileEntry
	Changelogs    []ChangelogEntry
}

func ParseRPM(r io.Reader) (*Package, error) {
	// Lead is 96 bytes
	if _, err := ParseLead(r); err != nil {
		return nil, fmt.Errorf("rpm: parsing lead: %w", err)
	}

	sigHeader, err := ParseHeader(r, 1<<24)
	if err != nil {
		return nil, fmt.Errorf("rpm: parsing signature header: %w", err)
	}

	padBytes := PadTo8(sigHeader.TotalSize())
	if padBytes > 0 {
		pad := make([]byte, padBytes)
		if _, err := io.ReadFull(r, pad); err != nil {
			return nil, fmt.Errorf("rpm: signature padding: %w", err)
		}
	}

	// Calculate header start offset: 96 (lead) + sigHeader.TotalSize() + padBytes
	headerStart := int64(96) + sigHeader.TotalSize() + padBytes

	pkgHeader, err := ParseHeader(r, 1<<24)
	if err != nil {
		return nil, fmt.Errorf("rpm: parsing package header: %w", err)
	}

	// Calculate header end offset: headerStart + pkgHeader.TotalSize()
	headerEnd := headerStart + pkgHeader.TotalSize()

	pkg := &Package{
		Name:        getString(pkgHeader, RPMTAG_NAME),
		Version:     getString(pkgHeader, RPMTAG_VERSION),
		Release:     getString(pkgHeader, RPMTAG_RELEASE),
		Epoch:       getUint32(pkgHeader, RPMTAG_EPOCH),
		Arch:        getString(pkgHeader, RPMTAG_ARCH),
		Summary:     getString(pkgHeader, RPMTAG_SUMMARY),
		Description: getString(pkgHeader, RPMTAG_DESCRIPTION),
		License:     getString(pkgHeader, RPMTAG_LICENSE),
		URL:         getString(pkgHeader, RPMTAG_URL),
		Vendor:      getString(pkgHeader, RPMTAG_VENDOR),
		Group:       getString(pkgHeader, RPMTAG_GROUP),
		Packager:    getString(pkgHeader, RPMTAG_PACKAGER),
		SourceRPM:   getString(pkgHeader, RPMTAG_SOURCERPM),
		BuildHost:   getString(pkgHeader, RPMTAG_BUILDHOST),
		BuildTime:   int64(getUint32(pkgHeader, RPMTAG_BUILDTIME)),
		PackageSize: int64(getUint32(pkgHeader, RPMTAG_SIZE)),
		HeaderStart: headerStart,
		HeaderEnd:   headerEnd,
	}

	pkg.Provides, _ = ParseDependencies(pkgHeader, RPMTAG_PROVIDES, RPMTAG_PROVIDEFLAGS, RPMTAG_PROVIDEVERSION)
	pkg.Requires, _ = ParseDependencies(pkgHeader, RPMTAG_REQUIRENAME, RPMTAG_REQUIREFLAGS, RPMTAG_REQUIREVERSION)
	pkg.Conflicts, _ = ParseDependencies(pkgHeader, RPMTAG_CONFLICTNAME, RPMTAG_CONFLICTFLAGS, RPMTAG_CONFLICTVERSION)
	pkg.Obsoletes, _ = ParseDependencies(pkgHeader, RPMTAG_OBSOLETENAME, RPMTAG_OBSOLETEFLAGS, RPMTAG_OBSOLETEVERSION)
	pkg.Suggests, _ = ParseDependencies(pkgHeader, RPMTAG_SUGGESTSNAME, RPMTAG_SUGGESTSFLAGS, TagType(0))
	pkg.Supplements, _ = ParseDependencies(pkgHeader, RPMTAG_SUPPLEMENTNAME, RPMTAG_SUPPLEMENTFLAGS, TagType(0))
	pkg.Enhances, _ = ParseDependencies(pkgHeader, RPMTAG_ENHANCENAME, RPMTAG_ENHANCEFLAGS, TagType(0))

	pkg.Files = extractFiles(pkgHeader)
	pkg.Changelogs = extractChangelogs(pkgHeader)

	return pkg, nil
}

func getString(h *Header, tag TagType) string {
	val, _ := h.GetString(tag)
	return val
}

func getUint32(h *Header, tag TagType) uint32 {
	val, _ := h.GetUint32(tag)
	return val
}

func extractFiles(h *Header) []FileEntry {
	dirs, _ := h.GetStringArray(RPMTAG_FILENAMES)
	if dirs == nil {
		return nil
	}

	sizes, _ := h.GetInt32s(RPMTAG_FILESIZES)
	modes, _ := h.GetInt32s(RPMTAG_FILEMODES)
	mtimes, _ := h.GetInt32s(RPMTAG_FILEMTIMES)
	digests, _ := h.GetStringArray(RPMTAG_FILEDIGESTS)
	linktos, _ := h.GetStringArray(RPMTAG_FILELINKTOS)

	files := make([]FileEntry, 0, len(dirs))
	for i, path := range dirs {
		entry := FileEntry{Path: path}

		if i < len(sizes) {
			entry.Size = int64(sizes[i])
		}

		if i < len(modes) {
			entry.Mode = uint16(modes[i] & 0xFFFF)
			entry.IsDir = (modes[i] & 0x4000) != 0
		}

		if i < len(mtimes) {
			entry.MTime = int64(mtimes[i])
		}

		if i < len(digests) {
			entry.Digest = digests[i]
		}

		if i < len(linktos) {
			entry.LinkTo = linktos[i]
		}

		files = append(files, entry)
	}

	return files
}

func extractChangelogs(h *Header) []ChangelogEntry {
	times, _ := h.GetInt32s(RPMTAG_CHANGELOGTIME)
	authors, _ := h.GetStringArray(RPMTAG_CHANGELOGNAME)
	texts, _ := h.GetStringArray(RPMTAG_CHANGELOGTEXT)

	if times == nil {
		return nil
	}

	entries := make([]ChangelogEntry, 0, len(times))
	for i := range times {
		entry := ChangelogEntry{Time: int64(times[i])}
		if i < len(authors) {
			entry.Author = authors[i]
		}

		if i < len(texts) {
			entry.Description = texts[i]
		}

		entries = append(entries, entry)
	}

	return entries
}

func (p *Package) GetMetadata() map[string]any {
	metadata := map[string]any{
		"format":        "rpm",
		"name":          p.Name,
		"version":       p.Version,
		"release":       p.Release,
		"epoch":         p.Epoch,
		"architecture":  p.Arch,
		"summary":       p.Summary,
		"description":   p.Description,
		"license":       p.License,
		"url":           p.URL,
		"vendor":        p.Vendor,
		"group":         p.Group,
		"packager":      p.Packager,
		"sourcerpm":     p.SourceRPM,
		"buildhost":     p.BuildHost,
		"buildtime":     p.BuildTime,
		"packagesize":   p.PackageSize,
		"installedsize": p.InstalledSize,
		"archivesize":   p.ArchiveSize,
	}

	if len(p.Provides) > 0 {
		metadata["provides"] = p.Provides
	}

	if len(p.Requires) > 0 {
		metadata["requires"] = p.Requires
	}

	if len(p.Conflicts) > 0 {
		metadata["conflicts"] = p.Conflicts
	}

	if len(p.Obsoletes) > 0 {
		metadata["obsoletes"] = p.Obsoletes
	}

	if len(p.Suggests) > 0 {
		metadata["suggests"] = p.Suggests
	}

	if len(p.Supplements) > 0 {
		metadata["supplements"] = p.Supplements
	}

	if len(p.Enhances) > 0 {
		metadata["enhances"] = p.Enhances
	}

	if len(p.Files) > 0 {
		metadata["filecount"] = len(p.Files)
	}

	if len(p.Changelogs) > 0 {
		metadata["changelogcount"] = len(p.Changelogs)
	}

	return metadata
}

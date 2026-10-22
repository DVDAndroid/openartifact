// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package apk inspects Android application packages: manifest, titles and
// icons from resources.arsc, signatures (v1 JAR, v2, v3, v3.1), native
// libraries, frameworks and embedded dependency metadata. It only reads; it
// never extracts to disk or uses entry names as paths.
package apk

import (
	"archive/zip"
	"crypto"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"
)

var (
	// ErrNotZip is returned for input that is not a zip archive.
	ErrNotZip = errors.New("apk: not a zip archive")
	// ErrNoManifest is returned when AndroidManifest.xml is missing or unreadable.
	ErrNoManifest = errors.New("apk: no readable AndroidManifest.xml")
)

// Options tune Inspect.
type Options struct {
	Limits Limits
	// Now is used for certificate expiry flags (default time.Now()).
	Now time.Time
	// SkipIconData leaves Icon.Data empty (metadata only).
	SkipIconData bool
	// SkipV1Entries skips hashing every entry for v1 verification.
	SkipV1Entries bool
}

// Info is everything Inspect learns about one APK.
type Info struct {
	Manifest      Manifest          `json:"manifest"`
	Label         string            `json:"label"`
	Labels        map[string]string `json:"labels"`
	LauncherLabel string            `json:"launcher_label,omitempty"`
	IconResource  string            `json:"icon_resource,omitempty"`
	Icons         []Icon            `json:"icons"`

	Signature     SignatureInfo `json:"signature"`
	ContentDigest string        `json:"content_digest"`

	ABIs         []string     `json:"abis"`
	NativeLibs   []NativeLib  `json:"native_libs"`
	PageSize16K  bool         `json:"page_size_16k"`
	Frameworks   []string     `json:"frameworks"`
	Dependencies []Dependency `json:"dependencies"`
	BuildInfo    BuildInfo    `json:"build_info"`
	Locales      []string     `json:"locales"`
	ZipChecks    ZipChecks    `json:"zip_checks"`
	Entries      []Entry      `json:"entries"`
	ManifestXML  string       `json:"-"`
	Warnings     []string     `json:"warnings"`
}

// Entry is one zip entry.
type Entry struct {
	Name             string `json:"name"`
	Method           uint16 `json:"method"`
	CRC32            uint32 `json:"crc32"`
	CompressedSize   int64  `json:"compressed_size"`
	UncompressedSize int64  `json:"size"`
	DataOffset       int64  `json:"-"`
}

// ZipChecks are packaging rules the platform or Play enforce.
type ZipChecks struct {
	Aligned4          bool     `json:"aligned_4"`
	Unaligned         []string `json:"unaligned,omitempty"`
	NativeAligned16K  bool     `json:"native_aligned_16k"`
	NativeCompressed  bool     `json:"native_compressed"`
	ArscStored        bool     `json:"resources_arsc_stored"`
	ArscAligned       bool     `json:"resources_arsc_aligned"`
	PrefixBytes       int64    `json:"prefix_bytes,omitempty"`
	DuplicateEntries  []string `json:"duplicate_entries,omitempty"`
	Problems          []string `json:"problems,omitempty"`
	EntryCount        int      `json:"entry_count"`
	UncompressedTotal int64    `json:"uncompressed_total"`
}

// SignatureInfo aggregates all schemes.
type SignatureInfo struct {
	Schemes  []string       `json:"schemes"`
	Verified bool           `json:"verified"`
	Results  []SchemeResult `json:"results"`
	// Signer is the certificate Android uses for the newest platforms
	// (v3.1 > v3 > v2 > v1), i.e. the app's current signing key.
	Signer  *CertInfo     `json:"signer,omitempty"`
	Signers []CertInfo    `json:"signers"`
	Lineage []LineageNode `json:"lineage,omitempty"`
	Blocks  []string      `json:"blocks,omitempty"`
	Flags   []string      `json:"flags,omitempty"`
	Errors  []string      `json:"errors,omitempty"`
}

// SignerSHA256 returns the current signer fingerprint, or "".
func (i *Info) SignerSHA256() string {
	if i.Signature.Signer == nil {
		return ""
	}

	return i.Signature.Signer.SHA256
}

type inspector struct {
	ra    io.ReaderAt
	size  int64
	opts  Options
	zr    *zip.Reader
	files map[string]*zip.File
	names []string
	res   *resTable
	info  *Info
}

func (in *inspector) warnf(format string, args ...any) {
	in.info.Warnings = append(in.info.Warnings, fmt.Sprintf(format, args...))
}

// Inspect analyses an APK read through ra. Malformed parts become warnings;
// only a non-zip input or a missing manifest is an error.
func Inspect(ra io.ReaderAt, size int64, opts Options) (*Info, error) {
	if opts.Limits == (Limits{}) {
		opts.Limits = DefaultLimits()
	}

	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	zr, err := zip.NewReader(ra, size)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotZip, err) //nolint:errorlint // one sentinel, detail as text
	}

	if len(zr.File) > opts.Limits.MaxEntries {
		return nil, fmt.Errorf("%w: %d zip entries", ErrLimit, len(zr.File))
	}

	in := &inspector{
		ra: ra, size: size, opts: opts, zr: zr,
		files: map[string]*zip.File{},
		info:  &Info{Labels: map[string]string{}},
	}

	in.indexEntries()

	if err := in.manifest(); err != nil {
		return nil, err
	}

	in.resources()
	in.signatures()
	in.nativeLibs()
	in.frameworksAndDeps()
	in.zipChecks()
	in.contentDigest()

	return in.info, nil
}

func (in *inspector) indexEntries() {
	var dups []string

	for _, f := range in.zr.File {
		if _, dup := in.files[f.Name]; dup {
			dups = append(dups, f.Name)
			continue
		}

		in.files[f.Name] = f
		in.names = append(in.names, f.Name)

		off, err := f.DataOffset()
		if err != nil {
			off = -1
		}

		in.info.Entries = append(in.info.Entries, Entry{
			Name:             f.Name,
			Method:           f.Method,
			CRC32:            f.CRC32,
			CompressedSize:   int64(f.CompressedSize64),   //nolint:gosec // zip sizes fit int64
			UncompressedSize: int64(f.UncompressedSize64), //nolint:gosec // zip sizes fit int64
			DataOffset:       off,
		})
	}

	if len(dups) > 0 {
		in.info.ZipChecks.DuplicateEntries = dups
		in.warnf("duplicate zip entries: %s", strings.Join(firstN(dups, 10), ", "))
	}
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}

	return s
}

func (in *inspector) manifest() error {
	f, ok := in.files["AndroidManifest.xml"]
	if !ok {
		return ErrNoManifest
	}

	b, err := readEntry(f, in.opts.Limits.MaxManifestSize)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoManifest, err) //nolint:errorlint // detail as text
	}

	root, ns, err := parseAXML(b, in.opts.Limits)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoManifest, err) //nolint:errorlint // detail as text
	}

	m, err := manifestFrom(root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoManifest, err) //nolint:errorlint // detail as text
	}

	in.info.Manifest = *m
	in.info.ManifestXML = renderXML(root, ns)

	return nil
}

func (in *inspector) resources() {
	m := &in.info.Manifest

	if f, ok := in.files["resources.arsc"]; ok {
		b, err := readEntry(f, in.opts.Limits.MaxArscSize)
		if err == nil {
			in.res, err = parseResTable(b, in.opts.Limits)
		}

		if err != nil {
			in.warnf("resources.arsc: %v", err)
		}
	} else if !m.IsSplit {
		in.warnf("resources.arsc missing")
	}

	in.info.Labels = in.labels(m.labelRef)

	switch {
	case in.info.Labels[""] != "":
		in.info.Label = in.info.Labels[""]
	case len(in.info.Labels) > 0:
		in.info.Label = in.info.Labels[sortedKeys(in.info.Labels)[0]]
	default:
		in.info.Label = m.Package
	}

	if l := in.labels(m.launcherLabel)[""]; l != "" && l != in.info.Label {
		in.info.LauncherLabel = l
	}

	in.info.Locales = in.locales()
	in.icons()
}

// labels resolves a label attribute to its per-locale strings.
func (in *inspector) labels(a Attr) map[string]string {
	out := map[string]string{}

	switch {
	case a.Value.isRef():
		best := map[string]Config{}

		for _, e := range in.res.resolveAll(a.Value.Data, 0) {
			if !e.value.isString() || e.value.Str == "" {
				continue
			}

			loc := e.cfg.Locale()
			if prev, seen := best[loc]; seen && plainer(prev, e.cfg) {
				continue
			}

			best[loc] = e.cfg
			out[loc] = e.value.Str
		}
	case a.Raw != "":
		out[""] = a.Raw
	case a.Value.isString() && a.Value.Str != "":
		out[""] = a.Value.Str
	}

	return out
}

// plainer reports whether a is a better default than b for the same locale.
func plainer(a, b Config) bool {
	if a.Other != b.Other {
		return !a.Other
	}

	return a.SDK <= b.SDK
}

func (in *inspector) locales() []string {
	if in.res == nil {
		return nil
	}

	seen := map[string]bool{}

	for _, p := range in.res.packages {
		for _, types := range p.types {
			for _, rt := range types {
				if l := rt.cfg.Locale(); l != "" {
					seen[l] = true
				}
			}
		}
	}

	return sortedKeys(seen)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

func (in *inspector) signatures() {
	sig := &in.info.Signature
	now := in.opts.Now

	l, err := findLayout(in.ra, in.size)
	if err != nil {
		sig.Errors = append(sig.Errors, err.Error())
		return
	}

	hasV2Plus := false

	switch err := l.readSigningBlock(in.ra); {
	case errors.Is(err, errNoSigBlock):
	case err != nil:
		sig.Errors = append(sig.Errors, "signing block: "+err.Error())
	default:
		hasV2Plus = in.signingBlock(l)
	}

	v1, stripped := verifyV1(in.files, in.names, in.v1Limits(), now, hasV2Plus)
	if v1 != nil {
		sig.Results = append([]SchemeResult{*v1}, sig.Results...)
	}

	sig.Errors = append(sig.Errors, stripped...)
	in.summarizeSignature()
}

func (in *inspector) v1Limits() Limits {
	lim := in.opts.Limits
	if in.opts.SkipV1Entries {
		lim.MaxV1Size = 0
	}

	return lim
}

// signingBlock parses the v2/v3/v3.1 entries and reports whether one exists.
func (in *inspector) signingBlock(l *zipLayout) bool {
	sig := &in.info.Signature

	pairs, order, err := l.pairs()
	if err != nil {
		sig.Errors = append(sig.Errors, "signing block: "+err.Error())
		return false
	}

	for _, id := range order {
		if name := blockName(id); name != "" {
			sig.Blocks = append(sig.Blocks, name)
		}
	}

	dc := &digestCache{l: l, ra: in.ra, sums: map[crypto.Hash][]byte{}, errs: map[crypto.Hash]error{}}
	found := false

	for _, s := range []struct {
		id   uint32
		name string
	}{{BlockV2, "v2"}, {BlockV3, "v3"}, {BlockV31, "v3.1"}} {
		if val, ok := pairs[s.id]; ok {
			sig.Results = append(sig.Results, parseV2V3(s.name, val, dc, in.opts.Now))
			found = true
		}
	}

	return found
}

func blockName(id uint32) string {
	switch id {
	case BlockV2, BlockV3, BlockV31:
		return ""
	case BlockVerityPad:
		return "verity_padding"
	case BlockSourceStamp, BlockStampV1:
		return "source_stamp"
	case BlockDependency:
		return "dependency_info"
	case BlockPlayFrost:
		return "play_frosting"
	}

	return fmt.Sprintf("0x%08x", id)
}

func (in *inspector) summarizeSignature() {
	sig := &in.info.Signature
	seen := map[string]bool{}
	sig.Verified = len(sig.Results) > 0 && len(sig.Errors) == 0
	sha1Only := len(sig.Results) > 0

	for _, r := range sig.Results {
		sig.Schemes = append(sig.Schemes, r.Scheme)
		sig.Verified = sig.Verified && r.Verified

		if len(r.Lineage) > 0 {
			sig.Lineage = r.Lineage
		}

		for _, s := range r.Signers {
			if s.Certificate == nil {
				continue
			}

			// Results are ordered v1, v2, v3, v3.1: the last one wins.
			sig.Signer = s.Certificate

			if !seen[s.Certificate.SHA256] {
				seen[s.Certificate.SHA256] = true
				sig.Signers = append(sig.Signers, *s.Certificate)
			}

			sha1Only = sha1Only && r.Scheme == "v1" && slices.Equal(s.DigestAlgorithms, []string{"SHA-1"})
		}
	}

	flags := map[string]bool{}
	for _, c := range sig.Signers {
		for _, f := range c.Flags {
			flags[f] = true
		}
	}

	if sha1Only {
		flags[FlagSHA1Only] = true
	}

	sig.Flags = sortedKeys(flags)

	m := in.info.Manifest
	if !slices.ContainsFunc(sig.Schemes, func(s string) bool { return s != "v1" }) && len(sig.Schemes) > 0 && m.TargetSDK >= 30 {
		in.info.ZipChecks.Problems = append(in.info.ZipChecks.Problems, "targetSdk >= 30 requires APK Signature Scheme v2 or newer")
	}

	if len(sig.Schemes) == 0 {
		in.warnf("APK is not signed")
	}
}

func (in *inspector) zipChecks() {
	zc := &in.info.ZipChecks
	zc.Aligned4, zc.NativeAligned16K, zc.ArscStored, zc.ArscAligned = true, true, false, false
	zc.EntryCount = len(in.info.Entries)

	var head [4]byte
	if _, err := in.ra.ReadAt(head[:], 0); err == nil && binary.LittleEndian.Uint32(head[:]) != 0x04034b50 {
		zc.PrefixBytes = in.firstHeaderOffset()
		in.warnf("data before the first zip entry (%d bytes)", zc.PrefixBytes)
	}

	for _, e := range in.info.Entries {
		zc.UncompressedTotal += e.UncompressedSize
		isLib := strings.HasPrefix(e.Name, "lib/") && strings.HasSuffix(e.Name, ".so")

		if isLib && e.Method != zip.Store {
			zc.NativeCompressed = true
		}

		if e.Method != zip.Store || e.DataOffset < 0 {
			continue
		}

		if e.DataOffset%4 != 0 {
			zc.Aligned4 = false
			zc.Unaligned = append(zc.Unaligned, e.Name)
		}

		if isLib && e.DataOffset%16384 != 0 {
			zc.NativeAligned16K = false
		}

		if e.Name == "resources.arsc" {
			zc.ArscStored, zc.ArscAligned = true, e.DataOffset%4 == 0
		}
	}

	zc.Unaligned = firstN(zc.Unaligned, 50)

	if m := in.info.Manifest; m.TargetSDK >= 30 && in.files["resources.arsc"] != nil && (!zc.ArscStored || !zc.ArscAligned) {
		zc.Problems = append(zc.Problems, "targetSdk >= 30 requires resources.arsc stored uncompressed and 4-byte aligned")
	}

	if !zc.Aligned4 {
		zc.Problems = append(zc.Problems, "uncompressed entries are not 4-byte aligned (run zipalign)")
	}

	in.info.PageSize16K = in.pageSize16K()
}

func (in *inspector) firstHeaderOffset() int64 {
	minOff := int64(-1)

	for _, e := range in.info.Entries {
		if e.DataOffset >= 0 && (minOff < 0 || e.DataOffset < minOff) {
			minOff = e.DataOffset
		}
	}

	if minOff < 0 {
		return 0
	}

	return minOff
}

// pageSize16K is true when every native library supports 16 KiB pages: its
// LOAD segments are 16 KiB aligned and, if stored, the entry is too.
func (in *inspector) pageSize16K() bool {
	if len(in.info.NativeLibs) == 0 {
		return true
	}

	for _, l := range in.info.NativeLibs {
		if l.LoadAlign > 0 && l.LoadAlign < 16384 {
			return false
		}

		if l.Stored && !l.Aligned16K {
			return false
		}
	}

	return true
}

// contentDigest hashes entry names, CRCs and sizes, leaving out signature
// files: two files with the same value are the same build signed differently.
func (in *inspector) contentDigest() {
	type row struct {
		name string
		crc  uint32
		size int64
	}

	rows := make([]row, 0, len(in.info.Entries))

	for _, e := range in.info.Entries {
		if isSignatureFile(e.Name) {
			continue
		}

		rows = append(rows, row{e.Name, e.CRC32, e.UncompressedSize})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	h := sha256.New()
	for _, r := range rows {
		fmt.Fprintf(h, "%s\x00%08x\x00%d\n", r.name, r.crc, r.size)
	}

	in.info.ContentDigest = hex.EncodeToString(h.Sum(nil))
}

// isSignatureFile reports whether name is part of a signature rather than
// of the signed content.
func isSignatureFile(name string) bool {
	dir, base, _ := strings.Cut(name, "/")
	if dir != "META-INF" || strings.Contains(base, "/") {
		return false
	}

	up := strings.ToUpper(base)
	if up == "MANIFEST.MF" || strings.HasPrefix(up, "STAMP-CERT-") {
		return true
	}

	for _, ext := range []string{".SF", ".RSA", ".DSA", ".EC"} {
		if strings.HasSuffix(up, ext) {
			return true
		}
	}

	return false
}

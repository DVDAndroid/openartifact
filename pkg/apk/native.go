// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"archive/zip"
	"bytes"
	"debug/elf"
	"encoding/hex"
	"io"
	"path"
	"sort"
	"strings"
)

// NativeLib is one lib/<abi>/*.so.
type NativeLib struct {
	ABI        string   `json:"abi"`
	Name       string   `json:"name"`
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	Stored     bool     `json:"stored"`
	Aligned16K bool     `json:"aligned_16k"`
	Class      string   `json:"class,omitempty"`
	Machine    string   `json:"machine,omitempty"`
	Soname     string   `json:"soname,omitempty"`
	Needed     []string `json:"needed,omitempty"`
	BuildID    string   `json:"build_id,omitempty"`
	Stripped   bool     `json:"stripped"`
	LoadAlign  uint64   `json:"load_align,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func (in *inspector) nativeLibs() {
	abis := map[string]bool{}
	budget := in.opts.Limits.MaxELFTotal

	for _, e := range in.info.Entries {
		dir, name := path.Split(e.Name)

		parts := strings.Split(strings.Trim(dir, "/"), "/")
		if len(parts) != 2 || parts[0] != "lib" || !strings.HasSuffix(name, ".so") {
			continue
		}

		abis[parts[1]] = true
		lib := NativeLib{
			ABI: parts[1], Name: name, Path: e.Name, Size: e.UncompressedSize,
			Stored: e.Method == zip.Store, Aligned16K: e.Method == zip.Store && e.DataOffset >= 0 && e.DataOffset%16384 == 0,
		}

		if budget-e.UncompressedSize < 0 || e.UncompressedSize > in.opts.Limits.MaxELFSize {
			lib.Error = "not analysed: size limit"
		} else {
			budget -= e.UncompressedSize
			in.readELF(&lib, in.files[e.Name], e)
		}

		in.info.NativeLibs = append(in.info.NativeLibs, lib)
	}

	in.info.ABIs = sortedKeys(abis)
}

func (in *inspector) readELF(lib *NativeLib, f *zip.File, e Entry) {
	var ra io.ReaderAt

	if lib.Stored && e.DataOffset >= 0 {
		ra = io.NewSectionReader(in.ra, e.DataOffset, e.UncompressedSize)
	} else {
		b, err := readEntry(f, in.opts.Limits.MaxELFSize)
		if err != nil {
			lib.Error = err.Error()
			return
		}

		ra = bytes.NewReader(b)
	}

	ef, err := elf.NewFile(ra)
	if err != nil {
		lib.Error = "not an ELF file"
		return
	}
	defer func() { _ = ef.Close() }()

	lib.Class = ef.Class.String()
	lib.Machine = ef.Machine.String()

	for _, p := range ef.Progs {
		if p.Type == elf.PT_LOAD && (lib.LoadAlign == 0 || p.Align < lib.LoadAlign) {
			lib.LoadAlign = p.Align
		}
	}

	if s, err := ef.DynString(elf.DT_SONAME); err == nil && len(s) > 0 {
		lib.Soname = s[0]
	}

	if n, err := ef.ImportedLibraries(); err == nil {
		sort.Strings(n)
		lib.Needed = n
	}

	lib.Stripped = ef.Section(".symtab") == nil
	lib.BuildID = buildID(ef)
}

func buildID(ef *elf.File) string {
	s := ef.Section(".note.gnu.build-id")
	if s == nil {
		return ""
	}

	b, err := s.Data()
	if err != nil || len(b) < 16 {
		return ""
	}

	nameSz := ef.ByteOrder.Uint32(b)
	descSz := ef.ByteOrder.Uint32(b[4:])
	start := 12 + (nameSz+3)&^3

	if uint64(start)+uint64(descSz) > uint64(len(b)) {
		return ""
	}

	return hex.EncodeToString(b[start : start+descSz])
}

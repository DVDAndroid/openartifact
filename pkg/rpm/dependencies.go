// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	SENSE_LESS         uint32 = 0x01
	SENSE_GREATER      uint32 = 0x02
	SENSE_EQUAL        uint32 = 0x04
	SENSE_ANY          uint32 = 0x08
	SENSE_LESSEQUAL    uint32 = SENSE_LESS | SENSE_EQUAL
	SENSE_GREATEREQUAL uint32 = SENSE_GREATER | SENSE_EQUAL
)

type Dependency struct {
	Name    string
	Flags   uint32
	Version string
	Epoch   uint32
}

func SenseFlagsString(flags uint32) string {
	if flags&SENSE_ANY != 0 || flags == 0 {
		return ""
	}

	parts := make([]string, 0, 2)

	if flags&SENSE_GREATER != 0 && flags&SENSE_EQUAL != 0 {
		return "GE"
	}

	if flags&SENSE_LESS != 0 && flags&SENSE_EQUAL != 0 {
		return "LE"
	}

	if flags&SENSE_GREATER != 0 {
		parts = append(parts, "GT")
	}

	if flags&SENSE_LESS != 0 {
		parts = append(parts, "LT")
	}

	if flags&SENSE_EQUAL != 0 {
		parts = append(parts, "EQ")
	}

	if len(parts) == 1 {
		return parts[0]
	}

	return strings.Join(parts, "&")
}

func ParseDependencies(header *Header, nameTag, flagsTag, versionTag TagType) ([]Dependency, error) {
	names, ok := header.GetStringArray(nameTag)
	if !ok || len(names) == 0 {
		return nil, nil
	}

	var flags []int32
	if flagsTag != 0 {
		flags, _ = header.GetInt32s(flagsTag)
	}

	var versions []string
	if versionTag != 0 {
		versions, _ = header.GetStringArray(versionTag)
	}

	deps := make([]Dependency, 0, len(names))
	for i, name := range names {
		dep := Dependency{Name: name}

		if i < len(flags) {
			dep.Flags = uint32(flags[i]) //nolint:gosec // RPM binary format requires specific integer widths
		}

		if i < len(versions) {
			dep.Version = versions[i]
			dep.Epoch, dep.Version = extractVersionEpoch(versions[i])
		}

		deps = append(deps, dep)
	}

	return deps, nil
}

func extractVersionEpoch(ver string) (uint32, string) {
	if before, after, ok := strings.Cut(ver, ":"); ok {
		e, err := strconv.ParseUint(before, 10, 32)
		if err == nil {
			return uint32(e), after
		}
	}

	return 0, ver
}

func formatEVR(epoch uint32, version, release string) string {
	if epoch > 0 {
		return fmt.Sprintf("%d:%s-%s", epoch, version, release)
	}

	return fmt.Sprintf("%s-%s", version, release)
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package rpm

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

type CompsGroup struct {
	ID          string
	Name        string
	Description string
	Default     bool
	UserVisible bool
	Packages    []CompsPackage
}

type CompsCategory struct {
	ID          string
	Name        string
	Description string
	GroupIDs    []string
}

type CompsPackage struct {
	Name      string
	Type      string // mandatory, default, optional, conditional
	Condition string // for conditional packages
}

type Comps struct {
	Groups     []CompsGroup
	Categories []CompsCategory
}

func ValidateCompsXML(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		_, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("rpm: invalid comps XML: %w", err)
		}
	}
}

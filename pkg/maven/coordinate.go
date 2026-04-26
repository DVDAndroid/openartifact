// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrInvalidCoordinate = errors.New("invalid maven coordinate")

var identifierRe = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type Coordinate struct {
	GroupID    string
	ArtifactID string
	Version    string
	Classifier string
	Extension  string
}

func ParseCoordinate(groupID, artifactID, version, classifier, extension string) (*Coordinate, error) {
	if !identifierRe.MatchString(groupID) {
		return nil, fmt.Errorf("%w: invalid groupID %q", ErrInvalidCoordinate, groupID)
	}

	if !identifierRe.MatchString(artifactID) {
		return nil, fmt.Errorf("%w: invalid artifactID %q", ErrInvalidCoordinate, artifactID)
	}

	if version == "" {
		return nil, fmt.Errorf("%w: version must not be empty", ErrInvalidCoordinate)
	}

	if extension == "" {
		return nil, fmt.Errorf("%w: extension must not be empty", ErrInvalidCoordinate)
	}

	for _, seg := range []string{groupID, artifactID, version, classifier, extension} {
		if strings.Contains(seg, "..") {
			return nil, fmt.Errorf("%w: segment is \"..\"", ErrInvalidCoordinate)
		}
	}

	return &Coordinate{
		GroupID:    groupID,
		ArtifactID: artifactID,
		Version:    version,
		Classifier: classifier,
		Extension:  extension,
	}, nil
}

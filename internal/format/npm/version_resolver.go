// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import (
	"context"
	"fmt"

	"github.com/openartifact/openartifact/internal/database"
)

type VersionResolver struct {
	db database.Database
}

func NewVersionResolver(db database.Database) *VersionResolver {
	return &VersionResolver{db: db}
}

func (vr *VersionResolver) Resolve(ctx context.Context, packageID, target string) (string, error) {
	tags, err := vr.db.GetNPMDistTags(ctx, packageID)
	if err != nil {
		return "", fmt.Errorf("get dist-tags: %w", err)
	}

	if version, ok := tags[target]; ok {
		return version, nil
	}

	v, err := vr.db.GetNPMVersion(ctx, packageID, target)
	if err != nil {
		return "", fmt.Errorf("get npm version: %w", err)
	}

	if v == nil {
		return "", fmt.Errorf("version not found: %s", target) //nolint:err113 // dynamic error with runtime context
	}

	return target, nil
}

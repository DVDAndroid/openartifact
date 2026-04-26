// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package maven

import (
	"context"
	"fmt"

	"github.com/openartifact/openartifact/internal/database"
	pkgmaven "github.com/openartifact/openartifact/pkg/maven"
)

type SnapshotResolver struct {
	db database.Database
}

func NewSnapshotResolver(db database.Database) *SnapshotResolver {
	return &SnapshotResolver{db: db}
}

func (s *SnapshotResolver) Resolve(ctx context.Context, repoID string, coord pkgmaven.Coordinate) (string, error) {
	artifact, err := s.db.GetLatestSnapshot(ctx, repoID, coord.GroupID, coord.ArtifactID, coord.Version)
	if err != nil {
		return "", fmt.Errorf("failed to resolve latest snapshot: %w", err)
	}

	if artifact == nil {
		return "", nil
	}

	return artifact.RepoPath, nil
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"slices"

	"github.com/openartifact/openartifact/internal/database/models"
)

// projectRoleActions is the single table of what a project role allows on the
// repositories of the project: the data-plane actions read, write and delete.
// No role includes "manage": project roles never make anybody a global admin.
//
//nolint:gochecknoglobals // immutable lookup table
var projectRoleActions = map[string][]string{
	models.ProjectRoleViewer:     {"read"},
	models.ProjectRoleDeveloper:  {"read", "write"},
	models.ProjectRoleMaintainer: {"read", "write", "delete"},
	models.ProjectRoleOwner:      {"read", "write", "delete"},
}

// projectRoleAllows reports whether role allows the data action.
func projectRoleAllows(role, action string) bool {
	return slices.Contains(projectRoleActions[role], action)
}

// maxProjectRole returns the higher-ranked of two roles; an unknown role never
// wins over a known one.
func maxProjectRole(a, b string) string {
	if models.ProjectRoleRank(b) > models.ProjectRoleRank(a) {
		return b
	}

	return a
}

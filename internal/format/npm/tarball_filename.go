// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package npm

import "strings"

func TarballFilename(packageName, version string) string {
	name := packageName
	if strings.HasPrefix(name, "@") {
		if idx := strings.Index(name, "/"); idx > 0 {
			name = name[idx+1:]
		}
	}

	return name + "-" + version + ".tgz"
}

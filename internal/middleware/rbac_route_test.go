// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package middleware

import (
	"net/http"
	"strings"
	"testing"
)

func TestExtractRepoAndPath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantRepo     string
		wantArtifact string
	}{
		// Generic /repos/{name}/... format-handler endpoints.
		{"npm packument", "/repos/npm-repo/lodash", "npm-repo", "lodash"},
		{"npm publish unscoped", "/repos/npm-repo/lodash", "npm-repo", "lodash"},
		{"npm scoped publish", "/repos/npm-repo/@scope%2Fname", "npm-repo", "@scope%2Fname"},
		{"npm dist-tags", "/repos/npm-repo/-/package/lodash/dist-tags/latest", "npm-repo", "-/package/lodash/dist-tags/latest"},
		{"pypi upload", "/repos/pypi-repo/legacy/", "pypi-repo", "legacy/"},
		{"pypi simple index", "/repos/pypi-repo/simple/", "pypi-repo", "simple/"},
		{"pypi package index", "/repos/pypi-repo/simple/requests/", "pypi-repo", "simple/requests/"},
		{"helm chart upload", "/repos/helm-repo/charts", "helm-repo", "charts"},
		{"helm index", "/repos/helm-repo/index.yaml", "helm-repo", "index.yaml"},
		{"bare repo path", "/repos/some-repo", "some-repo", ""},

		// Isolated /repos/<marker>/{name}/... namespaces.
		{"raw namespace", "/repos/raw/my-raw-repo/some/nested/path.txt", "my-raw-repo", "some/nested/path.txt"},
		{"raw namespace bare", "/repos/raw/my-raw-repo", "my-raw-repo", ""},
		{"maven namespace", "/repos/maven/my-maven-repo/com/example/lib/1.0/lib-1.0.jar", "my-maven-repo", "com/example/lib/1.0/lib-1.0.jar"},
		{"sbt namespace", "/repos/sbt/my-sbt-repo/com/example/lib_2.13/1.0/lib.jar", "my-sbt-repo", "com/example/lib_2.13/1.0/lib.jar"},
		{"ivy namespace", "/repos/ivy/my-ivy-repo/org/module/ivys/ivy-1.0.xml", "my-ivy-repo", "org/module/ivys/ivy-1.0.xml"},

		// Docker/OCI /v2/... routes: name may itself contain "/".
		{"docker manifest simple", "/v2/myimage/manifests/latest", "myimage", "manifests/latest"},
		{"docker manifest namespaced", "/v2/org/team/myimage/manifests/sha256:abcd", "org/team/myimage", "manifests/sha256:abcd"},
		{"docker blob", "/v2/myimage/blobs/sha256:deadbeef", "myimage", "blobs/sha256:deadbeef"},
		{"docker blob upload init", "/v2/myimage/blobs/uploads/", "myimage", "blobs/uploads/"},
		{"docker blob upload chunk", "/v2/myimage/blobs/uploads/abc-123", "myimage", "blobs/uploads/abc-123"},
		{"docker tags list", "/v2/myimage/tags/list", "myimage", "tags/list"},
		{"docker referrers", "/v2/myimage/referrers/sha256:deadbeef", "myimage", "referrers/sha256:deadbeef"},
		{"docker manifest name contains literal marker segment", "/v2/myrepo/manifests/latest/manifests/sha256:abcd", "myrepo/manifests/latest", "manifests/sha256:abcd"},
		{"docker version check", "/v2/", "", ""},

		// Non-repo API paths.
		{"admin api path", "/api/v1/admin/users", "", "/api/v1/admin/users"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRepo, gotArtifact := extractRepoAndPath(tc.path)
			if gotRepo != tc.wantRepo || gotArtifact != tc.wantArtifact {
				t.Errorf("extractRepoAndPath(%q) = (%q, %q), want (%q, %q)",
					tc.path, gotRepo, gotArtifact, tc.wantRepo, tc.wantArtifact)
			}
		})
	}
}

func TestIsSPARepoPathAndBypass(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantBypass bool
	}{
		// Legitimate Vue-router SPA pages — must bypass RBAC.
		{"repo list", "/repos", true},
		{"new repo wizard", "/repos/new", true},
		{"repo detail", "/repos/myrepo", true},
		{"repo edit", "/repos/myrepo/edit", true},
		{"repo browse", "/repos/myrepo/browse", true},
		{"repo graph", "/repos/myrepo/graph", true},
		{"package detail", "/repos/myrepo/packages/npm/lodash", true},

		// Real format-handler endpoints that must NOT bypass (a path-segment
		// count heuristic would wrongly bypass these).
		{"npm packument", "/repos/myrepo/lodash", false},
		{"npm publish", "/repos/myrepo/lodash", false},
		{"pypi upload", "/repos/myrepo/legacy/", false},
		{"pypi simple index", "/repos/myrepo/simple/", false},
		{"helm chart upload", "/repos/myrepo/charts", false},
		{"helm index", "/repos/myrepo/index.yaml", false},
		{"rpm repomd", "/repos/myrepo/repodata/repomd.xml", false},

		// Isolated namespaces are never SPA pages, regardless of shape.
		{"raw bare", "/repos/raw/myrepo", false},
		{"maven bare", "/repos/maven/myrepo", false},
		{"sbt bare", "/repos/sbt/myrepo", false},
		{"ivy bare", "/repos/ivy/myrepo", false},

		// Docker/OCI and /api/ paths are always excluded from the /repos/
		// SPA-bypass branch entirely (handled by the prefix checks, not
		// isSPARepoPath), verified here via the same bypass decision used
		// in RBACMiddleware.
		{"docker manifest", "/v2/myimage/manifests/latest", false},
		{"admin api", "/api/v1/admin/users", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isRepoSPA := strings.HasPrefix(tc.path, "/repos/") &&
				isSPARepoPath(strings.Split(strings.Trim(tc.path, "/"), "/"))

			bypass := !strings.HasPrefix(tc.path, "/api/") && !strings.HasPrefix(tc.path, "/v2/") &&
				(!strings.HasPrefix(tc.path, "/repos/") || isRepoSPA)

			if bypass != tc.wantBypass {
				t.Errorf("bypass decision for %q = %v, want %v", tc.path, bypass, tc.wantBypass)
			}
		})
	}
}

func TestMethodToActionDockerPaths(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/v2/myimage/manifests/latest", "read"},
		{http.MethodHead, "/v2/myimage/blobs/sha256:deadbeef", "read"},
		{http.MethodPut, "/v2/myimage/manifests/latest", "write"},
		{http.MethodPost, "/v2/myimage/blobs/uploads/", "write"},
		{http.MethodDelete, "/v2/myimage/manifests/latest", "delete"},
	}

	for _, tc := range tests {
		if got := methodToAction(tc.method, tc.path); got != tc.want {
			t.Errorf("methodToAction(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

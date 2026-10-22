// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import "testing"

func TestCheckSigner(t *testing.T) {
	const key = "aa"

	tests := []struct {
		name, policy, origin, trust string
		known                       bool
		signer                      string
		wantReject, wantWarn        bool
	}{
		{"own build, own key", "warn", "built", "mine", true, key, false, false},
		{"patched with an unknown key warns", "warn", "patched", "", false, key, false, true},
		{"patched with the publisher's key warns", "warn", "patched", "original", true, key, false, true},
		{"patched with an unknown key under reject", "reject", "patched", "", false, key, true, true},
		{"patched with an unknown key under allow", "allow", "patched", "", false, key, false, false},
		{"original with a new key is fine under warn", "warn", "original", "", false, key, false, true},
		{"original with a known key is fine", "warn", "original", "original", true, key, false, false},
		{"original with a new key under reject", "reject", "original", "", false, key, true, true},
		{"untrusted key warns even for originals", "warn", "original", "untrusted", true, key, false, true},
		{"untrusted key under allow is accepted", "allow", "built", "untrusted", true, key, false, false},
		{"unsigned files always warn", "allow", "built", "", false, "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkSigner(tt.policy, tt.origin, tt.trust, tt.known, tt.signer)
			if got.Reject != tt.wantReject || (got.Warning != "") != tt.wantWarn {
				t.Errorf("checkSigner = %+v, want reject=%v warning=%v", got, tt.wantReject, tt.wantWarn)
			}
		})
	}
}

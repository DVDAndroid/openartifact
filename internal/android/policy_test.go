// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import "testing"

func TestCheckSigner(t *testing.T) {
	const key = "aa"

	tests := []struct {
		name                 string
		facts                signerFacts
		wantReject, wantWarn bool
	}{
		{"own build, own key", signerFacts{Policy: "warn", Origin: "built", Signer: key, Known: true, Trust: "mine"}, false, false},
		{"patched with an unknown key warns", signerFacts{Policy: "warn", Origin: "patched", Signer: key}, false, true},
		{"patched with the publisher's key warns", signerFacts{Policy: "warn", Origin: "patched", Signer: key, Known: true, Trust: "original"}, false, true},
		{"patched with an unknown key under reject", signerFacts{Policy: "reject", Origin: "patched", Signer: key}, true, true},
		{"patched with an unknown key under allow", signerFacts{Policy: "allow", Origin: "patched", Signer: key}, false, false},
		{"first original of an app is silent", signerFacts{Policy: "warn", Origin: "original", Signer: key}, false, false},
		{"first original under reject is accepted", signerFacts{Policy: "reject", Origin: "original", Signer: key}, false, false},
		{"original with a known key is fine", signerFacts{Policy: "warn", Origin: "original", Signer: key, Known: true, Trust: "original"}, false, false},
		{"original with a new key for a signed app warns", signerFacts{Policy: "warn", Origin: "original", Signer: key, AppHasOtherKey: true}, false, true},
		{"original with a new key under reject", signerFacts{Policy: "reject", Origin: "original", Signer: key, AppHasOtherKey: true}, true, true},
		{"untrusted key warns even for originals", signerFacts{Policy: "warn", Origin: "original", Signer: key, Known: true, Trust: "untrusted"}, false, true},
		{"untrusted key under allow is accepted", signerFacts{Policy: "allow", Origin: "built", Signer: key, Known: true, Trust: "untrusted"}, false, false},
		{"unsigned files always warn", signerFacts{Policy: "allow", Origin: "built"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkSigner(tt.facts)
			if got.Reject != tt.wantReject || (got.Warning != "") != tt.wantWarn {
				t.Errorf("checkSigner = %+v, want reject=%v warning=%v", got, tt.wantReject, tt.wantWarn)
			}
		})
	}
}

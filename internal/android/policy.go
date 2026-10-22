// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import "github.com/openartifact/openartifact/internal/database/models"

// signerCheck is how an upload's signing key is judged against the app's policy.
type signerCheck struct {
	// Reject means the upload is refused with 409 (unless the client accepts the new signer).
	Reject  bool
	Warning string
}

// signerFacts is what the repository knows about an upload's signing key.
type signerFacts struct {
	Policy string
	Origin string
	Signer string
	// Known is true for a key registered in the repository (globally or for the app); Trust is its level.
	Known bool
	Trust string
	// AppHasOtherKey is true when the app already has versions signed with a different key.
	AppHasOtherKey bool
}

// checkSigner applies the signer policy of an app to an upload.
//
// Builds the owner makes (built, patched) are expected to carry a key marked as
// theirs. Anything else (originals, unknown provenance) comes with the
// publisher's key, so a key never seen before is only worth a word when the
// app was signed with a different one until now. A key marked untrusted is
// always reported.
func checkSigner(f signerFacts) signerCheck {
	if f.Signer == "" {
		return signerCheck{Warning: "the file is not signed"}
	}

	var problem string

	switch {
	case f.Known && f.Trust == models.AndroidTrustUntrusted:
		problem = "signed with a key marked untrusted"
	case f.Origin == models.AndroidOriginBuilt || f.Origin == models.AndroidOriginPatched:
		if !f.Known || f.Trust != models.AndroidTrustMine {
			problem = "a " + f.Origin + " file is not signed with a key marked as yours"
		}
	case !f.Known && f.AppHasOtherKey:
		problem = "signed with a key this app has not been signed with before"
	}

	if problem == "" {
		return signerCheck{}
	}

	switch f.Policy {
	case models.AndroidPolicyAllow:
		return signerCheck{}
	case models.AndroidPolicyReject:
		return signerCheck{Reject: true, Warning: problem}
	default:
		return signerCheck{Warning: problem}
	}
}

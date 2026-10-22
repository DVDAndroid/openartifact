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

// checkSigner applies the signer policy of an app to an upload.
//
// Builds the owner makes (built, patched) are expected to carry a key marked as
// theirs; originals keep the publisher's key, so any key is fine for them
// unless the app rejects unknown signers. A key marked untrusted is always
// reported. known is false for a key the repository has never seen.
func checkSigner(policy, origin, trust string, known bool, signer string) signerCheck {
	if signer == "" {
		return signerCheck{Warning: "the file is not signed"}
	}

	var problem string

	switch {
	case known && trust == models.AndroidTrustUntrusted:
		problem = "signed with a key marked untrusted"
	case origin == models.AndroidOriginBuilt || origin == models.AndroidOriginPatched:
		if !known || trust != models.AndroidTrustMine {
			problem = "a " + origin + " file is not signed with a key marked as yours"
		}
	case !known:
		problem = "signed with a key this repository has not seen"
	}

	if problem == "" {
		return signerCheck{}
	}

	switch policy {
	case models.AndroidPolicyAllow:
		return signerCheck{}
	case models.AndroidPolicyReject:
		return signerCheck{Reject: true, Warning: problem}
	default:
		return signerCheck{Warning: problem}
	}
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package android

import (
	"encoding/json"
	"net/http"

	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/internal/urlparams"
	"github.com/openartifact/openartifact/pkg/apk"
)

const maxLabelLen = 255

func validTrust(t string) bool {
	switch t {
	case models.AndroidTrustMine, models.AndroidTrustOriginal, models.AndroidTrustThirdParty, models.AndroidTrustUntrusted:
		return true
	}

	return false
}

func (h *Handler) handleListSigners(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	appID := ""

	if pkg := r.URL.Query().Get("app"); pkg != "" {
		app, err := h.store.GetAndroidApp(r.Context(), repo.ID, pkg)
		if err != nil {
			h.fail(w, "failed to look up the app", err)

			return
		}

		if app == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "app not found")

			return
		}

		appID = app.ID
	}

	signers, err := h.store.ListAndroidSigners(r.Context(), repo.ID, appID)
	if err != nil {
		h.fail(w, "failed to list signers", err)

		return
	}

	if signers == nil {
		signers = []models.AndroidSigner{}
	}

	writeOK(w, http.StatusOK, signers)
}

// signerBody is the body of POST signers. Only public data is accepted: a
// fingerprint or a certificate, never a keystore or a private key.
type signerBody struct {
	SHA256      string `json:"sha256"`
	Certificate string `json:"certificate"`
	Label       string `json:"label"`
	Trust       string `json:"trust"`
	Notes       string `json:"notes"`
	App         string `json:"app"`
}

// resolve returns the fingerprint and the certificate JSON (if any) the body names.
func (b *signerBody) resolve() (string, json.RawMessage, error) {
	var (
		sha  string
		cert json.RawMessage
	)

	if b.Certificate != "" {
		ci, err := apk.ParseCertificateFile([]byte(b.Certificate))
		if err != nil {
			return "", nil, inputErrorf("certificate is not a valid PEM or DER certificate: %v", err)
		}

		if cert, err = json.Marshal(ci); err != nil {
			return "", nil, inputErrorf("certificate cannot be recorded: %v", err)
		}

		sha = ci.SHA256
	}

	if b.SHA256 != "" {
		norm, ok := apk.NormalizeFingerprint(b.SHA256)
		if !ok {
			return "", nil, inputError("sha256 must be a SHA-256 fingerprint")
		}

		if sha != "" && sha != norm {
			return "", nil, inputError("sha256 does not match the certificate")
		}

		sha = norm
	}

	if sha == "" {
		return "", nil, inputError("send a sha256 fingerprint or a certificate")
	}

	return sha, cert, nil
}

func (h *Handler) handleCreateSigner(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	var body signerBody
	if err := decodeJSON(w, r, &body); err != nil {
		h.writeInputError(w, err, "invalid body")

		return
	}

	sha, cert, err := body.resolve()
	if err == nil && !validTrust(body.Trust) {
		err = inputError("trust must be mine, original, third_party or untrusted")
	}

	if err == nil && len(body.Label) > maxLabelLen {
		err = inputErrorf("label is longer than %d bytes", maxLabelLen)
	}

	if err == nil {
		err = checkText("notes", body.Notes)
	}

	if err != nil {
		h.writeInputError(w, err, "invalid body")

		return
	}

	s := &models.AndroidSigner{RepositoryID: repo.ID, SHA256: sha, Label: body.Label, Trust: body.Trust, Notes: body.Notes, Certificate: cert}

	if body.App != "" {
		app, err := h.store.GetAndroidApp(r.Context(), repo.ID, body.App)
		if err != nil {
			h.fail(w, "failed to look up the app", err)

			return
		}

		if app == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "app not found")

			return
		}

		s.AppID = app.ID
	}

	if err := h.store.UpsertAndroidSigner(r.Context(), s); err != nil {
		h.fail(w, "failed to save the signer", err)

		return
	}

	writeOK(w, http.StatusCreated, s)
}

// signerUpdateBody is the body of PUT signers/{id}; absent fields stay unchanged.
type signerUpdateBody struct {
	Label *string `json:"label"`
	Trust *string `json:"trust"`
	Notes *string `json:"notes"`
}

func (h *Handler) handleUpdateSigner(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	var body signerUpdateBody
	if err := decodeJSON(w, r, &body); err != nil {
		h.writeInputError(w, err, "invalid body")

		return
	}

	known, err := h.store.ListAndroidSigners(r.Context(), repo.ID, "")
	if err != nil {
		h.fail(w, "failed to list signers", err)

		return
	}

	id := urlparams.Vars(r)["id"]

	var cur *models.AndroidSigner

	for i := range known {
		if known[i].ID == id {
			cur = &known[i]
		}
	}

	if cur == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "signer not found")

		return
	}

	label, trust, notes := cur.Label, cur.Trust, cur.Notes

	if body.Label != nil {
		label = *body.Label
	}

	if body.Trust != nil {
		trust = *body.Trust
	}

	if body.Notes != nil {
		notes = *body.Notes
	}

	if !validTrust(trust) || len(label) > maxLabelLen || checkText("notes", notes) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "trust must be mine, original, third_party or untrusted; label and notes must be short")

		return
	}

	updated, err := h.store.UpdateAndroidSigner(r.Context(), repo.ID, id, label, trust, notes)
	if err != nil {
		h.fail(w, "failed to update the signer", err)

		return
	}

	if updated == nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "signer not found")

		return
	}

	writeOK(w, http.StatusOK, updated)
}

func (h *Handler) handleDeleteSigner(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, true)
	if repo == nil {
		return
	}

	ok, err := h.store.DeleteAndroidSigner(r.Context(), repo.ID, urlparams.Vars(r)["id"])
	if err != nil {
		h.fail(w, "failed to delete the signer", err)

		return
	}

	if !ok {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "signer not found")

		return
	}

	writeOK(w, http.StatusOK, map[string]any{"signer_id": urlparams.Vars(r)["id"]})
}

func (h *Handler) handleListTags(w http.ResponseWriter, r *http.Request) {
	repo := h.repoOf(w, r, false)
	if repo == nil {
		return
	}

	tags, err := h.store.ListAndroidTags(r.Context(), repo.ID)
	if err != nil {
		h.fail(w, "failed to list tags", err)

		return
	}

	if tags == nil {
		tags = []models.AndroidTagCount{}
	}

	writeOK(w, http.StatusOK, tags)
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"time"
)

type SSOProvider struct {
	ID                 string            `json:"id,omitempty"`
	Slug               string            `json:"slug"`
	Kind               string            `json:"kind"`
	DisplayName        string            `json:"display_name"`
	Enabled            bool              `json:"enabled"`
	OIDCConfig         json.RawMessage   `json:"oidc_config,omitempty"`
	SAMLConfig         json.RawMessage   `json:"saml_config,omitempty"`
	ClaimMapping       json.RawMessage   `json:"claim_mapping,omitempty"`
	GroupMapping       map[string]string `json:"group_mapping,omitempty"`
	PermissionMapping  map[string]string `json:"permission_mapping,omitempty"`
	AutoProvisionUsers bool              `json:"auto_provision_users"`
	LinkByEmail        bool              `json:"link_by_email"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type OIDCConfig struct {
	Issuer        string   `json:"issuer"`
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	Scopes        []string `json:"scopes"`
	UsernameClaim string   `json:"username_claim"`
	EmailClaim    string   `json:"email_claim"`
	NameClaim     string   `json:"name_claim"`
	GroupsClaim   string   `json:"groups_claim"`
}

type SAMLConfig struct {
	IDPMetadataURL    string `json:"idp_metadata_url"`
	IDPMetadataXML    string `json:"idp_metadata_xml"`
	UsernameAttribute string `json:"username_attribute"`
	EmailAttribute    string `json:"email_attribute"`
	NameAttribute     string `json:"name_attribute"`
	GroupsAttribute   string `json:"groups_attribute"`
}

type ClaimMapping struct {
	UsernameClaim string `json:"username_claim"`
	EmailClaim    string `json:"email_claim"`
	NameClaim     string `json:"name_claim"`
	GroupsClaim   string `json:"groups_claim"`
}

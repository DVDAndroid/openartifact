// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package errors

// Error codes used throughout the API.
// All codes are lowercase snake_case for consistency.
const (
	// ErrInternal is returned for general unrecoverable server errors.
	ErrInternal         = "internal_error"
	ErrNotFound         = "not_found"
	ErrBadRequest       = "bad_request"
	ErrUnauthorized     = "unauthorized"
	ErrForbidden        = "forbidden"
	ErrValidationFailed = "validation_failed"
	ErrInvalidRequest   = "invalid_request"
	ErrNotImplemented   = "not_implemented"

	// ErrPasswordChangeRequired is returned when the user must change their password before proceeding.
	ErrPasswordChangeRequired = "password_change_required"
	ErrAuthEvaluationFailed   = "auth_evaluation_failed"
	ErrCSRFProtection         = "csrf_protection"
	ErrRateLimitExceeded      = "rate_limit_exceeded"
	ErrIPNotAllowed           = "ip_not_allowed"
	ErrAccountDisabled        = "account_disabled"

	// ErrInvalidInput is returned when the uploaded artifact fails validation.
	ErrInvalidInput  = "invalid_input"
	ErrUploadError   = "upload_error"
	ErrDatabaseError = "database_error"
	ErrStorageError  = "storage_error"

	// ErrProviderNotFound is returned when the requested SSO provider does not exist.
	ErrProviderNotFound       = "provider_not_found"
	ErrIDPMetadataUnavailable = "idp_metadata_unavailable"
	ErrSAMLSignatureInvalid   = "saml_signature_invalid"
	ErrSAMLAudienceMismatch   = "saml_audience_mismatch"
	ErrSAMLAssertionExpired   = "saml_assertion_expired"
	ErrSAMLInvalid            = "saml_invalid"
	ErrMissingSubject         = "missing_subject"
	ErrUserNotProvisioned     = "user_not_provisioned"
	ErrEmailAlreadyInUse      = "email_already_in_use"
	ErrEmailLinkAmbiguous     = "email_link_ambiguous"
	ErrInvalidRelayState      = "invalid_relay_state"
	ErrOIDCDiscoveryFailed    = "oidc_discovery_failed"
	ErrLoadFailed             = "load_failed"
	ErrUnauthenticated        = "unauthenticated"
)

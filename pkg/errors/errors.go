// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package errors provides structured error types for OpenArtifact.
//
// Inspired by the zot project (https://github.com/project-zot/zot):
// errors wrap a sentinel error and carry a key-value detail map that
// survives the error chain, enabling rich context at log/response time.
//
// Usage:
//
//	return errors.NewError(errors.ErrNotFound).AddDetail("repo", name)
//
//	if errors.Is(err, errors.ErrNotFound) { ... }
//
//	if details := errors.GetDetails(err); details["repo"] != "" { ... }
package errors

import (
	"errors"
	"fmt"
	"maps"
)

// Error wraps a sentinel error and carries structured key-value details.
// Details survive wrapping via errors.As, enabling rich context at any
// point in the call stack without losing the original sentinel identity.
type Error struct {
	err     error
	details map[string]string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.err.Error()
}

// Is delegates to the wrapped sentinel so errors.Is works transparently.
func (e *Error) Is(target error) bool {
	return errors.Is(e.err, target)
}

// Unwrap exposes the wrapped error for errors.As / errors.Is traversal.
func (e *Error) Unwrap() error {
	return e.err
}

// AddDetail attaches a key-value pair to the error and returns itself
// for fluent chaining: errors.NewError(ErrNotFound).AddDetail("id", id).
func (e *Error) AddDetail(key, value string) *Error {
	e.details[key] = value
	return e
}

// GetDetails returns the detail map (never nil).
func (e *Error) GetDetails() map[string]string {
	return e.details
}

// NewError wraps a sentinel error, preserving any details already
// present if err is itself an *Error.
func NewError(err error) *Error {
	return &Error{
		err:     err,
		details: GetDetails(err),
	}
}

// NewErrorf wraps a formatted error message as a new sentinel.
func NewErrorf(format string, args ...any) *Error {
	return &Error{
		err:     fmt.Errorf(format, args...), //nolint:err113 // this IS the error construction helper
		details: make(map[string]string),
	}
}

// GetDetails extracts the detail map from any error in the chain.
// Returns an empty (non-nil) map if no *Error is found.
func GetDetails(err error) map[string]string {
	if e, ok := errors.AsType[*Error](err); ok {
		// Copy so callers can't mutate the original.
		out := make(map[string]string, len(e.details))
		maps.Copy(out, e.details)

		return out
	}

	return make(map[string]string)
}

// Is is a convenience re-export so callers don't need to import stdlib errors.
//
//nolint:gochecknoglobals // stdlib re-export for package-level convenience
var Is = errors.Is

// As is a convenience re-export.
//
//nolint:gochecknoglobals // stdlib re-export for package-level convenience
var As = errors.As

// New is a convenience re-export for creating simple sentinel errors.
//
//nolint:gochecknoglobals // stdlib re-export for package-level convenience
var New = errors.New

// Wrap wraps err with additional context message (fmt.Errorf %w style).
func Wrap(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf wraps err with a formatted context message.
func Wrapf(err error, format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, err)...) //nolint:err113 // this IS the error construction helper
}

// ---- Sentinel errors -------------------------------------------------------
// Grouped by domain. Use NewError(ErrXxx).AddDetail(...) to attach context.

// Generic.
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrInternal      = errors.New("internal error")
	ErrConflict      = errors.New("conflict")
	ErrTimeout       = errors.New("timeout")
	ErrUnavailable   = errors.New("service unavailable")
)

// Storage / blob.
var (
	ErrBlobNotFound     = errors.New("blob not found")
	ErrBlobExists       = errors.New("blob already exists")
	ErrStorageBackend   = errors.New("storage backend error")
	ErrChecksumMismatch = errors.New("checksum mismatch")
)

// Repository / package.
var (
	ErrRepoNotFound    = errors.New("repository not found")
	ErrRepoExists      = errors.New("repository already exists")
	ErrPackageNotFound = errors.New("package not found")
	ErrFormatInvalid   = errors.New("invalid package format")
	ErrFormatNotFound  = errors.New("format handler not found")
)

// Auth / SSO.
var (
	ErrAuthFailed          = errors.New("authentication failed")
	ErrTokenInvalid        = errors.New("invalid token")
	ErrTokenExpired        = errors.New("token expired")
	ErrSessionNotFound     = errors.New("session not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrUserDisabled        = errors.New("user account disabled")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrSSOProviderNotFound = errors.New("SSO provider not found")
)

// Database.
var (
	ErrDBConnection  = errors.New("database connection error")
	ErrDBQuery       = errors.New("database query error")
	ErrDBTransaction = errors.New("database transaction error")
	ErrDBMigration   = errors.New("database migration error")
)

// Configuration.
var (
	ErrConfigInvalid = errors.New("invalid configuration")
	ErrConfigMissing = errors.New("missing required configuration")
)

// ---- ErrorCode helpers -----------------------------------------------------
// Prefer the sentinel errors above for new code.

// ErrorCode is a string error code.
type ErrorCode = string

const (
	ErrCodeInvalidInput    ErrorCode = "INVALID_INPUT"
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrCodeAlreadyExists   ErrorCode = "ALREADY_EXISTS"
	ErrCodeFormatNotFound  ErrorCode = "FORMAT_NOT_FOUND"
	ErrCodeInternalError   ErrorCode = "INTERNAL_ERROR"
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden       ErrorCode = "FORBIDDEN"
	ErrCodeConflict        ErrorCode = "CONFLICT"
	ErrCodeValidationError ErrorCode = "VALIDATION_ERROR"
)

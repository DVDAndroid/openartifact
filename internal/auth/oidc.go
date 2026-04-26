// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
)

var (
	ErrSSOProviderNotFound = errors.New("sso provider not found")
	ErrOIDCDiscoveryFailed = errors.New("oidc discovery failed")
	ErrIDTokenMissing      = errors.New("id token missing")
	ErrIDTokenExpired      = errors.New("id token expired")
)

//nolint:gochecknoglobals // process-lifetime OIDC provider cache
var oidcCache sync.Map

type OIDCProvider struct {
	Slug          string
	Provider      *gooidc.Provider
	Verifier      *gooidc.IDTokenVerifier
	Config        oauth2.Config
	Cfg           models.OIDCConfig
	UpdatedAt     time.Time
	EndSessionURL string
}

func LoadOIDCProvider(ctx context.Context, db database.Database, slug, baseURL string) (*OIDCProvider, error) {
	row, err := db.GetSSOProviderBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get sso provider: %w", err)
	}

	if row == nil {
		return nil, ErrSSOProviderNotFound
	}

	if row.Kind != "oidc" {
		return nil, fmt.Errorf("provider %q is not an oidc provider (kind=%s)", slug, row.Kind) //nolint:err113 // dynamic error with runtime context
	}

	var cfg models.OIDCConfig
	if err := json.Unmarshal(row.OIDCConfig, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal oidc_config: %w", err)
	}

	if cached, ok := oidcCache.Load(slug); ok {
		if p, ok := cached.(*OIDCProvider); ok {
			if p.UpdatedAt.Equal(row.UpdatedAt) {
				return p, nil
			}
		}
	}

	provider, err := gooidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc_discovery_failed: %w", ErrOIDCDiscoveryFailed)
	}

	var pc struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}

	_ = provider.Claims(&pc)

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  baseURL + "/api/v1/auth/sso/oidc/" + slug + "/callback",
		Scopes:       ensureOpenID(cfg.Scopes),
	}

	p := &OIDCProvider{
		Slug:          slug,
		Provider:      provider,
		Verifier:      provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID}),
		Config:        oauth2Cfg,
		Cfg:           cfg,
		UpdatedAt:     row.UpdatedAt,
		EndSessionURL: pc.EndSessionEndpoint,
	}

	oidcCache.Store(slug, p)

	return p, nil
}

func ensureOpenID(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{"openid", "profile", "email", "groups"}
	}

	if slices.Contains(scopes, "openid") {
		return scopes
	}

	return append([]string{"openid"}, scopes...)
}

func (p *OIDCProvider) AuthURL(state, nonce, codeChallenge string) string {
	return p.Config.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		gooidc.Nonce(nonce),
	)
}

func (p *OIDCProvider) Exchange(ctx context.Context, code, codeVerifier string) (*gooidc.IDToken, map[string]any, error) {
	token, err := p.Config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, nil, fmt.Errorf("code exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, nil, ErrIDTokenMissing
	}

	idToken, err := p.Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		if IsUnknownKeyError(err) {
			oidcCache.Delete(p.Slug)
		}

		return nil, nil, fmt.Errorf("verify id token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, nil, fmt.Errorf("decode claims: %w", err)
	}

	return idToken, claims, nil
}

// IsUnknownKeyError detects JWKS key-rotation errors from go-oidc.
// go-oidc errors are unstructured strings; best-effort string matching is used.
func IsUnknownKeyError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "unknown kid") ||
		strings.Contains(msg, "key not found") ||
		strings.Contains(msg, "failed to verify id token")
}

func (p *OIDCProvider) EndSession(idTokenHint, postLogoutRedirectURI string) (string, bool) {
	if p.EndSessionURL == "" {
		return "", false
	}

	u, err := url.Parse(p.EndSessionURL)
	if err != nil {
		return "", false
	}

	q := u.Query()
	q.Set("id_token_hint", idTokenHint)
	q.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	u.RawQuery = q.Encode()

	return u.String(), true
}

func ExchangeWithRetry(ctx context.Context, db database.Database, slug, baseURL, code, codeVerifier string) (*gooidc.IDToken, map[string]any, *OIDCProvider, error) {
	p, err := LoadOIDCProvider(ctx, db, slug, baseURL)
	if err != nil {
		return nil, nil, nil, err
	}

	idToken, claims, err := p.Exchange(ctx, code, codeVerifier)
	if err == nil {
		return idToken, claims, p, nil
	}

	if !IsUnknownKeyError(err) {
		return nil, nil, p, err
	}

	oidcCache.Delete(slug)

	p2, err2 := LoadOIDCProvider(ctx, db, slug, baseURL)
	if err2 != nil {
		return nil, nil, nil, err2
	}

	idToken, claims, err = p2.Exchange(ctx, code, codeVerifier)

	return idToken, claims, p2, err
}

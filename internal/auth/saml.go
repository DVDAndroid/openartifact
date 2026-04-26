// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"

	"github.com/openartifact/openartifact/internal/config"
	"github.com/openartifact/openartifact/internal/database"
	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

var (
	ErrIDPMetadataUnavailable = errors.New("idp metadata unavailable")
	ErrSAMLSignatureInvalid   = errors.New("saml signature invalid")
	ErrSAMLAudienceMismatch   = errors.New("saml audience mismatch")
	ErrSAMLAssertionExpired   = errors.New("saml assertion expired")
)

//nolint:gochecknoglobals // process-lifetime SAML provider cache
var samlCache sync.Map

//nolint:gochecknoglobals // process-lifetime SAML provider cache
var samlMetadataCache sync.Map

type cachedMetadata struct {
	descriptor *saml.EntityDescriptor
	fetchedAt  time.Time
}

type SAMLProvider struct {
	Slug     string
	Cfg      models.SAMLConfig
	SP       *samlsp.Middleware
	EntityID string
	AcsURL   string
}

func LoadSAMLProvider(ctx context.Context, db database.Database, slug, baseURL string, appCfg *config.Config, log *logger.Logger) (*SAMLProvider, error) {
	row, err := db.GetSSOProviderBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get sso provider: %w", err)
	}

	if row == nil {
		return nil, ErrSSOProviderNotFound
	}

	if row.Kind != "saml" {
		return nil, fmt.Errorf("provider %q is not a saml provider (kind=%s)", slug, row.Kind) //nolint:err113 // dynamic error with runtime context
	}

	var samlCfg models.SAMLConfig
	if err := json.Unmarshal(row.SAMLConfig, &samlCfg); err != nil {
		return nil, fmt.Errorf("unmarshal saml_config: %w", err)
	}

	entityID := baseURL + "/api/v1/auth/sso/saml/" + slug + "/metadata"
	acsURL := baseURL + "/api/v1/auth/sso/saml/" + slug + "/acs"

	idpMeta, err := fetchIDPMetadata(ctx, samlCfg, log)
	if err != nil {
		return nil, err
	}

	cacheKey := slug + "|" + idpMeta.EntityID
	if cached, ok := samlCache.Load(cacheKey); ok {
		if p, ok := cached.(*SAMLProvider); ok {
			return p, nil
		}
	}

	rsaKey, cert, err := loadOrGenerateSAMLKeypair(appCfg, log)
	if err != nil {
		return nil, fmt.Errorf("load or generate sp keypair: %w", err)
	}

	acsURLParsed, err := url.Parse(acsURL)
	if err != nil {
		return nil, fmt.Errorf("parse acs url: %w", err)
	}

	opts := samlsp.Options{
		EntityID:          entityID,
		URL:               *acsURLParsed,
		Key:               rsaKey,
		Certificate:       cert,
		IDPMetadata:       idpMeta,
		AllowIDPInitiated: false,
	}

	sp, err := samlsp.New(opts)
	if err != nil {
		return nil, fmt.Errorf("create saml middleware: %w", err)
	}

	sp.ServiceProvider.AcsURL = *acsURLParsed
	sp.ServiceProvider.MetadataURL = *mustParseURL(entityID)

	p := &SAMLProvider{
		Slug:     slug,
		Cfg:      samlCfg,
		SP:       sp,
		EntityID: entityID,
		AcsURL:   acsURL,
	}

	samlCache.Store(cacheKey, p)

	return p, nil
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("invalid url %q: %v", raw, err))
	}

	return u
}

func generateSelfSignedCert() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "openartifact-saml-sp"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}

func loadOrGenerateSAMLKeypair(cfg *config.Config, log *logger.Logger) (*rsa.PrivateKey, *x509.Certificate, error) {
	// If both cert and key are configured via environment variables, load them
	if cfg.Auth.SAMLSPCertPEM != "" && cfg.Auth.SAMLSPKeyPEM != "" { //nolint:nestif // SAML cert loading requires nested validation
		_, err := tls.X509KeyPair([]byte(cfg.Auth.SAMLSPCertPEM), []byte(cfg.Auth.SAMLSPKeyPEM))
		if err != nil {
			return nil, nil, fmt.Errorf("parse saml sp keypair from config: %w", err)
		}

		// Parse the private key
		keyBlock, _ := pem.Decode([]byte(cfg.Auth.SAMLSPKeyPEM))
		if keyBlock == nil {
			return nil, nil, errors.New("failed to decode SAML SP private key PEM") //nolint:err113 // static error
		}

		privKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			// Try PKCS8 format
			parsedKey, err2 := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
			if err2 != nil {
				return nil, nil, fmt.Errorf("parse saml sp private key: %w", err)
			}

			var ok bool

			privKey, ok = parsedKey.(*rsa.PrivateKey)
			if !ok {
				return nil, nil, errors.New("saml sp private key is not an RSA key") //nolint:err113 // static error
			}
		}

		// Parse the certificate
		certBlock, _ := pem.Decode([]byte(cfg.Auth.SAMLSPCertPEM))
		if certBlock == nil {
			return nil, nil, errors.New("failed to decode SAML SP certificate PEM") //nolint:err113 // static error
		}

		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("parse saml sp certificate: %w", err)
		}

		if log != nil {
			log.Infow("Loaded SAML SP keypair from environment variables", "cert_subject", cert.Subject.String())
		}

		return privKey, cert, nil
	}

	// Generate ephemeral keypair and log warning
	if log != nil {
		log.Warnw("SAML SP keypair not configured via environment variables; using ephemeral keypair. " +
			"Set OPENARTIFACT_SAML_SP_CERT and OPENARTIFACT_SAML_SP_KEY for persistence across restarts. " +
			"Generate with: openssl req -x509 -newkey rsa:2048 -keyout sp.key -out sp.crt -days 3650 -nodes -subj \"/CN=openartifact-sp\"")
	}

	return generateSelfSignedCert()
}

func fetchIDPMetadata(ctx context.Context, cfg models.SAMLConfig, log *logger.Logger) (*saml.EntityDescriptor, error) {
	if cfg.IDPMetadataURL != "" {
		return fetchIDPMetadataFromURL(ctx, cfg.IDPMetadataURL, log)
	}

	if cfg.IDPMetadataXML != "" {
		cacheKey := "static:" + cfg.IDPMetadataXML
		if cached, ok := samlMetadataCache.Load(cacheKey); ok {
			if cm, ok := cached.(*cachedMetadata); ok {
				return cm.descriptor, nil
			}
		}

		meta, err := samlsp.ParseMetadata([]byte(cfg.IDPMetadataXML))
		if err != nil {
			return nil, fmt.Errorf("parse idp metadata xml: %w", err)
		}

		samlMetadataCache.Store(cacheKey, &cachedMetadata{descriptor: meta, fetchedAt: time.Now()})

		return meta, nil
	}

	return nil, errors.New("no idp metadata source configured") //nolint:err113 // static error
}

func fetchIDPMetadataFromURL(ctx context.Context, metadataURL string, log *logger.Logger) (*saml.EntityDescriptor, error) {
	const cacheTTL = time.Hour

	if cached, ok := samlMetadataCache.Load(metadataURL); ok {
		if entry, ok := cached.(*cachedMetadata); ok {
			if time.Since(entry.fetchedAt) < cacheTTL {
				return entry.descriptor, nil
			}
		}
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return fallbackCachedMetadata(metadataURL, fmt.Errorf("build metadata request: %w", err), log)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fallbackCachedMetadata(metadataURL, err, log)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fallbackCachedMetadata(metadataURL, err, log)
	}

	meta, err := samlsp.ParseMetadata(body)
	if err != nil {
		return fallbackCachedMetadata(metadataURL, err, log)
	}

	samlMetadataCache.Store(metadataURL, &cachedMetadata{descriptor: meta, fetchedAt: time.Now()})

	return meta, nil
}

func fallbackCachedMetadata(metadataURL string, fetchErr error, log *logger.Logger) (*saml.EntityDescriptor, error) {
	if cached, ok := samlMetadataCache.Load(metadataURL); ok {
		if log != nil {
			log.Warnw("SAML IdP metadata URL unreachable; using cached copy", "url", metadataURL, "error", fetchErr)
		}

		if cm, ok := cached.(*cachedMetadata); ok {
			return cm.descriptor, nil
		}
	}

	return nil, fmt.Errorf("%w: %w", ErrIDPMetadataUnavailable, fetchErr)
}

func (p *SAMLProvider) LoginURL(relayState string) (string, error) {
	redirectURL, err := p.SP.ServiceProvider.MakeRedirectAuthenticationRequest(relayState)
	if err != nil {
		return "", fmt.Errorf("make redirect authn request: %w", err)
	}

	return redirectURL.String(), nil
}

func (p *SAMLProvider) ParseCallback(r *http.Request) (*saml.Assertion, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}

	assertion, err := p.SP.ServiceProvider.ParseResponse(r, nil)
	if err != nil {
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "signature"):
			return nil, fmt.Errorf("%w: %w", ErrSAMLSignatureInvalid, err)
		case strings.Contains(msg, "audience"):
			return nil, fmt.Errorf("%w: %w", ErrSAMLAudienceMismatch, err)
		case strings.Contains(msg, "expired") || strings.Contains(msg, "notonorafter"):
			return nil, fmt.Errorf("%w: %w", ErrSAMLAssertionExpired, err)
		}

		return nil, err
	}

	return assertion, nil
}

func (p *SAMLProvider) MetadataXML() ([]byte, error) {
	metadata := p.SP.ServiceProvider.Metadata()

	buf, err := xml.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sp metadata: %w", err)
	}

	out := append([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"), buf...)

	return out, nil
}

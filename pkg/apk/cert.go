// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"bytes"
	"crypto/dsa" //nolint:staticcheck // old APKs are still DSA-signed; only verified, never generated
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/md5" //nolint:gosec // MD5 fingerprint is displayed, never trusted
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SHA-1 fingerprint is displayed, never trusted
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"strings"
	"time"
)

// Certificate flags.
const (
	FlagDebugCert     = "debug_certificate"
	FlagAOSPTestKey   = "aosp_test_key"
	FlagWeakRSA       = "rsa_under_2048"
	FlagExpired       = "expired"
	FlagExpiresBefore = "expires_before_2033"
	FlagSHA1Only      = "sha1_only"
)

// expiryHorizon is Google Play's minimum certificate validity end
// (22 October 2033); certificates ending earlier are flagged.
//
//nolint:gochecknoglobals // constant date
var expiryHorizon = time.Date(2033, 10, 22, 0, 0, 0, 0, time.UTC)

// aospTestSubject is the subject shared by the AOSP testkey, platform,
// shared, media and networkstack keys.
const aospTestSubject = "EMAILADDRESS=android@android.com,CN=Android,OU=Android,O=Android,L=Mountain View,ST=California,C=US"

// CertInfo describes one X.509 signing certificate.
type CertInfo struct {
	SHA256             string    `json:"sha256"`
	SHA1               string    `json:"sha1"`
	MD5                string    `json:"md5"`
	PublicKeySHA256    string    `json:"public_key_sha256"`
	Subject            string    `json:"subject"`
	Issuer             string    `json:"issuer"`
	Serial             string    `json:"serial"`
	NotBefore          time.Time `json:"not_before"`
	NotAfter           time.Time `json:"not_after"`
	PublicKeyAlgorithm string    `json:"public_key_algorithm"`
	PublicKeySize      int       `json:"public_key_size"`
	SignatureAlgorithm string    `json:"signature_algorithm"`
	SelfSigned         bool      `json:"self_signed"`
	Flags              []string  `json:"flags,omitempty"`

	cert *x509.Certificate
}

func hexSum(b []byte, h func([]byte) []byte) string {
	return hex.EncodeToString(h(b))
}

// CertInfoFromDER describes a DER certificate. The fingerprints are those of
// the DER bytes, as apksigner, F-Droid and AppVerifier print them.
func CertInfoFromDER(der []byte, now time.Time) (*CertInfo, error) {
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}

	ci := &CertInfo{
		SHA256:             hexSum(der, func(b []byte) []byte { s := sha256.Sum256(b); return s[:] }),
		SHA1:               hexSum(der, func(b []byte) []byte { s := sha1.Sum(b); return s[:] }), //nolint:gosec // display only
		MD5:                hexSum(der, func(b []byte) []byte { s := md5.Sum(b); return s[:] }),  //nolint:gosec // display only
		PublicKeySHA256:    hexSum(c.RawSubjectPublicKeyInfo, func(b []byte) []byte { s := sha256.Sum256(b); return s[:] }),
		Subject:            c.Subject.String(),
		Issuer:             c.Issuer.String(),
		Serial:             c.SerialNumber.Text(16),
		NotBefore:          c.NotBefore.UTC(),
		NotAfter:           c.NotAfter.UTC(),
		PublicKeyAlgorithm: c.PublicKeyAlgorithm.String(),
		PublicKeySize:      publicKeySize(c.PublicKey),
		SignatureAlgorithm: c.SignatureAlgorithm.String(),
		SelfSigned:         bytes.Equal(c.RawSubject, c.RawIssuer),
		cert:               c,
	}

	ci.Flags = certFlags(ci, now)

	return ci, nil
}

func certFlags(ci *CertInfo, now time.Time) []string {
	var flags []string

	if strings.Contains(ci.Subject, "CN=Android Debug") {
		flags = append(flags, FlagDebugCert)
	}

	if ci.Subject == aospTestSubject {
		flags = append(flags, FlagAOSPTestKey)
	}

	if ci.PublicKeyAlgorithm == "RSA" && ci.PublicKeySize < 2048 {
		flags = append(flags, FlagWeakRSA)
	}

	if !now.IsZero() && ci.NotAfter.Before(now) {
		flags = append(flags, FlagExpired)
	}

	if ci.NotAfter.Before(expiryHorizon) {
		flags = append(flags, FlagExpiresBefore)
	}

	return flags
}

func publicKeySize(pub any) int {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		return k.N.BitLen()
	case *ecdsa.PublicKey:
		return k.Curve.Params().BitSize
	case *dsa.PublicKey:
		return k.P.BitLen()
	case ed25519.PublicKey:
		return 256
	}

	return 0
}

var errNoCertificate = errors.New("no certificate found")

// ParseCertificateFile reads a public certificate in PEM or DER form, as
// exported by keytool -exportcert (with or without -rfc).
func ParseCertificateFile(b []byte) (*CertInfo, error) {
	b = bytes.TrimSpace(b)

	if blk, _ := pem.Decode(b); blk != nil {
		if blk.Type != "CERTIFICATE" {
			return nil, errNoCertificate
		}

		b = blk.Bytes
	}

	return CertInfoFromDER(b, time.Now())
}

// NormalizeFingerprint turns "AB:CD:..." or "abcd..." into lowercase hex and
// reports whether it is a SHA-256 fingerprint.
func NormalizeFingerprint(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, " "); i >= 0 { // "SHA256: AB:CD..." from keytool -list -v
		s = s[i+1:]
	}

	s = strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(s))
	if len(s) != 64 {
		return "", false
	}

	if _, err := hex.DecodeString(s); err != nil {
		return "", false
	}

	return s, true
}

// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"time"
)

// Object identifiers used by JAR signature blocks.
//
//nolint:gochecknoglobals // constant OIDs
var (
	oidSignedData    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidMessageDigest = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSHA1          = asn1.ObjectIdentifier{1, 3, 14, 3, 2, 26}
	oidSHA256        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidSHA384        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	oidSHA512        = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 3}
)

type pkcs7ContentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
}

type pkcs7SignedData struct {
	Version          int
	DigestAlgorithms asn1.RawValue
	ContentInfo      asn1.RawValue
	Certificates     asn1.RawValue     `asn1:"optional,tag:0"`
	CRLs             asn1.RawValue     `asn1:"optional,tag:1"`
	SignerInfos      []pkcs7SignerInfo `asn1:"set"`
}

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type pkcs7SignerInfo struct {
	Version            int
	SID                asn1.RawValue
	DigestAlgorithm    algorithmIdentifier
	AuthAttributes     asn1.RawValue `asn1:"optional,tag:0"`
	DigestEncAlgorithm algorithmIdentifier
	EncryptedDigest    []byte
	UnauthAttributes   asn1.RawValue `asn1:"optional,tag:1"`
}

type pkcs7Attribute struct {
	Type   asn1.ObjectIdentifier
	Values asn1.RawValue `asn1:"set"`
}

var errV1 = errors.New("v1 signature")

func oidHash(oid asn1.ObjectIdentifier) crypto.Hash {
	switch {
	case oid.Equal(oidSHA1):
		return crypto.SHA1
	case oid.Equal(oidSHA256):
		return crypto.SHA256
	case oid.Equal(oidSHA384):
		return crypto.SHA384
	case oid.Equal(oidSHA512):
		return crypto.SHA512
	}

	return 0
}

// splitSections splits raw manifest bytes on blank lines, keeping the line
// terminators of each section (digests cover them).
func splitSections(b []byte) [][]byte {
	var (
		out   [][]byte
		start int
	)

	for i := 0; i < len(b); {
		end := bytes.IndexByte(b[i:], '\n')
		if end < 0 {
			break
		}

		line := b[i : i+end+1]
		i += end + 1

		if len(bytes.TrimRight(line, "\r\n")) == 0 {
			out = append(out, b[start:i])
			start = i
		}
	}

	if start < len(b) {
		out = append(out, b[start:])
	}

	return out
}

// sectionAttrs parses "Name: value" lines with 72-byte continuation lines.
func sectionAttrs(sec []byte) map[string]string {
	attrs := map[string]string{}

	var lastKey string

	for line := range strings.SplitSeq(strings.ReplaceAll(string(sec), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, " ") && lastKey != "" {
			attrs[lastKey] += line[1:]
			continue
		}

		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}

		lastKey = k
		attrs[k] = v
	}

	return attrs
}

func readEntry(f *zip.File, limit int64) ([]byte, error) {
	if int64(f.UncompressedSize64) > limit { //nolint:gosec // compared against a positive limit
		return nil, fmt.Errorf("%w: %s is %d bytes", ErrLimit, f.Name, f.UncompressedSize64)
	}

	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	return io.ReadAll(io.LimitReader(rc, limit+1))
}

// verifyV1 checks the JAR signature of the APK. files maps entry names to
// their zip entries (first occurrence).
func verifyV1(files map[string]*zip.File, names []string, lim Limits, now time.Time, hasV2Plus bool) (*SchemeResult, []string) {
	var blocks []string

	for _, n := range names {
		dir, base := path.Split(n)
		if dir != "META-INF/" {
			continue
		}

		switch strings.ToUpper(path.Ext(base)) {
		case ".RSA", ".DSA", ".EC":
			blocks = append(blocks, n)
		}
	}

	if len(blocks) == 0 {
		return nil, nil
	}

	res := &SchemeResult{Scheme: "v1"}

	mf, ok := files["META-INF/MANIFEST.MF"]
	if !ok {
		res.Errors = append(res.Errors, "META-INF/MANIFEST.MF missing")
		return res, nil
	}

	mfBytes, err := readEntry(mf, lim.MaxMetaFileSize)
	if err != nil {
		res.Errors = append(res.Errors, "MANIFEST.MF: "+err.Error())
		return res, nil
	}

	var strippedFor []string

	for _, blk := range blocks {
		s, stripped := verifyV1Signer(files, blk, mfBytes, lim, now, hasV2Plus)
		res.Signers = append(res.Signers, s)
		strippedFor = append(strippedFor, stripped...)
	}

	res.Errors = append(res.Errors, verifyV1Entries(files, names, mfBytes, lim)...)
	res.Verified = len(res.Errors) == 0

	for _, s := range res.Signers {
		res.Verified = res.Verified && s.Verified
	}

	return res, strippedFor
}

func verifyV1Signer(files map[string]*zip.File, blk string, mfBytes []byte, lim Limits, now time.Time, hasV2Plus bool) (Signer, []string) {
	var s Signer

	sfName := strings.TrimSuffix(blk, path.Ext(blk)) + ".SF"

	sf, ok := files[sfName]
	if !ok {
		s.Errors = append(s.Errors, sfName+" missing")
		return s, nil
	}

	sfBytes, err := readEntry(sf, lim.MaxMetaFileSize)
	if err != nil {
		s.Errors = append(s.Errors, err.Error())
		return s, nil
	}

	blkBytes, err := readEntry(files[blk], lim.MaxMetaFileSize)
	if err != nil {
		s.Errors = append(s.Errors, err.Error())
		return s, nil
	}

	if err := verifyPKCS7(&s, blkBytes, sfBytes, now); err != nil {
		s.Errors = append(s.Errors, err.Error())
	}

	sections := splitSections(sfBytes)
	if len(sections) == 0 {
		s.Errors = append(s.Errors, sfName+" is empty")
		return s, nil
	}

	main := sectionAttrs(sections[0])

	var stripped []string

	// X-Android-APK-Signed lists the newer schemes the APK was signed with;
	// their absence means the v2+ block was stripped (downgrade attack).
	if v := main["X-Android-APK-Signed"]; v != "" && !hasV2Plus {
		stripped = append(stripped, "v1 signature declares APK Signature Scheme "+v+" but the signing block is missing")
	}

	if !checkManifestDigest(main, mfBytes) {
		s.Errors = append(s.Errors, sfName+": manifest digest does not match MANIFEST.MF")
	}

	s.Verified = len(s.Errors) == 0

	return s, stripped
}

// checkManifestDigest verifies the whole-manifest digest of a .SF main section.
func checkManifestDigest(main map[string]string, mf []byte) bool {
	found := false

	for k, v := range main {
		alg, ok := strings.CutSuffix(k, "-Digest-Manifest")
		if !ok {
			continue
		}

		h := hashFor(alg)
		if h == nil {
			continue
		}

		found = true

		h.Write(mf)

		if base64.StdEncoding.EncodeToString(h.Sum(nil)) != v {
			return false
		}
	}

	return found
}

// verifyV1Entries checks every MANIFEST.MF entry digest against the file
// content, and that every non-META-INF entry is listed.
func verifyV1Entries(files map[string]*zip.File, names []string, mfBytes []byte, lim Limits) []string {
	listed := map[string]map[string]string{}

	for _, sec := range splitSections(mfBytes)[1:] {
		a := sectionAttrs(sec)
		if n := a["Name"]; n != "" {
			listed[n] = a
		}
	}

	var (
		errs   []string
		budget = lim.MaxV1Size
	)

	for _, n := range names {
		if strings.HasPrefix(n, "META-INF/") || strings.HasSuffix(n, "/") {
			continue
		}

		a, ok := listed[n]
		if !ok {
			errs = append(errs, n+" is not in MANIFEST.MF")
			continue
		}

		f := files[n]
		budget -= int64(f.UncompressedSize64) //nolint:gosec // bounded by archive

		if budget < 0 {
			errs = append(errs, "v1 entry digests not checked: size limit reached")
			break
		}

		if err := checkEntryDigest(f, a); err != nil {
			errs = append(errs, err.Error())
		}

		if len(errs) > 20 {
			break
		}
	}

	return errs
}

func checkEntryDigest(f *zip.File, attrs map[string]string) error {
	for k, want := range attrs {
		alg, ok := strings.CutSuffix(k, "-Digest")
		if !ok {
			continue
		}

		h := hashFor(alg)
		if h == nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}

		_, err = io.Copy(h, rc) //nolint:gosec // bounded by the v1 size budget
		_ = rc.Close()

		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}

		if base64.StdEncoding.EncodeToString(h.Sum(nil)) != want {
			return fmt.Errorf("%s: digest does not match MANIFEST.MF: %w", f.Name, errV1)
		}

		return nil
	}

	return fmt.Errorf("%s: no supported digest in MANIFEST.MF: %w", f.Name, errV1)
}

// verifyPKCS7 checks the PKCS#7 signature block over the .SF content and
// records the signer certificate.
func verifyPKCS7(s *Signer, blk, content []byte, now time.Time) error {
	var ci pkcs7ContentInfo
	if _, err := asn1.Unmarshal(blk, &ci); err != nil || !ci.ContentType.Equal(oidSignedData) {
		return fmt.Errorf("%w: not a PKCS#7 SignedData block", errV1)
	}

	var sd pkcs7SignedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		return fmt.Errorf("%w: SignedData: %v", errV1, err) //nolint:errorlint // detail only
	}

	certs, err := parseCertSet(sd.Certificates.Bytes, now)
	if err != nil || len(certs) == 0 {
		return fmt.Errorf("%w: no certificate", errV1)
	}

	if len(sd.SignerInfos) == 0 {
		return fmt.Errorf("%w: no signer info", errV1)
	}

	si := sd.SignerInfos[0]
	cert := matchSignerCert(certs, si)
	s.Certificate = cert

	for _, c := range certs {
		if c != cert {
			s.Chain = append(s.Chain, *c)
		}
	}

	h := oidHash(si.DigestAlgorithm.Algorithm)
	if h == 0 {
		return fmt.Errorf("%w: unsupported digest algorithm %v", errV1, si.DigestAlgorithm.Algorithm)
	}

	s.DigestAlgorithms = append(s.DigestAlgorithms, h.String())
	signed := content

	if len(si.AuthAttributes.FullBytes) > 0 {
		var err error
		if signed, err = checkAuthAttrs(si, h, content); err != nil {
			return err
		}
	}

	hh := h.New()
	hh.Write(signed)

	return verifyDigestSig(cert.cert.PublicKey, h, hh.Sum(nil), si.EncryptedDigest)
}

func parseCertSet(b []byte, now time.Time) ([]*CertInfo, error) {
	var out []*CertInfo

	for len(b) > 0 {
		var raw asn1.RawValue

		rest, err := asn1.Unmarshal(b, &raw)
		if err != nil {
			return out, err
		}

		ci, err := CertInfoFromDER(raw.FullBytes, now)
		if err != nil {
			return out, err
		}

		out = append(out, ci)
		b = rest
	}

	return out, nil
}

// matchSignerCert picks the certificate named by the signer's issuer and
// serial number, falling back to the first one.
func matchSignerCert(certs []*CertInfo, si pkcs7SignerInfo) *CertInfo {
	var ias struct {
		Issuer asn1.RawValue
		Serial asn1.RawValue
	}

	if _, err := asn1.Unmarshal(si.SID.FullBytes, &ias); err == nil {
		for _, c := range certs {
			if bytes.Equal(c.cert.RawIssuer, ias.Issuer.FullBytes) && bytes.Equal(c.cert.SerialNumber.Bytes(), trimSerial(ias.Serial.Bytes)) {
				return c
			}
		}
	}

	return certs[0]
}

func trimSerial(b []byte) []byte {
	for len(b) > 1 && b[0] == 0 {
		b = b[1:]
	}

	return b
}

// checkAuthAttrs verifies the messageDigest attribute and returns the bytes
// actually signed (the attributes re-tagged as a SET).
func checkAuthAttrs(si pkcs7SignerInfo, h crypto.Hash, content []byte) ([]byte, error) {
	var attrs []pkcs7Attribute
	if _, err := asn1.UnmarshalWithParams(si.AuthAttributes.FullBytes, &attrs, "set,tag:0"); err != nil {
		return nil, fmt.Errorf("%w: authenticated attributes", errV1)
	}

	hh := h.New()
	hh.Write(content)
	sum := hh.Sum(nil)

	ok := slices.ContainsFunc(attrs, func(a pkcs7Attribute) bool {
		if !a.Type.Equal(oidMessageDigest) {
			return false
		}

		var d []byte
		_, err := asn1.Unmarshal(a.Values.Bytes, &d)

		return err == nil && bytes.Equal(d, sum)
	})
	if !ok {
		return nil, fmt.Errorf("%w: messageDigest does not match the .SF file", errV1)
	}

	signed := bytes.Clone(si.AuthAttributes.FullBytes)
	signed[0] = 0x31 // [0] IMPLICIT -> SET OF

	return signed, nil
}

func verifyDigestSig(pub any, h crypto.Hash, sum, sig []byte) error {
	switch k := pub.(type) {
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(k, h, sum, sig); err != nil {
			return fmt.Errorf("%w: RSA signature does not verify", errV1)
		}

		return nil
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(k, sum, sig) {
			return fmt.Errorf("%w: ECDSA signature does not verify", errV1)
		}

		return nil
	}

	if err := verifyDSA(pub, sum, sig); err != nil {
		return fmt.Errorf("%w: signature does not verify", errV1)
	}

	return nil
}

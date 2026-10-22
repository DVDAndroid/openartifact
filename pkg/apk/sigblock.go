// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"bytes"
	"crypto"
	"crypto/dsa" //nolint:staticcheck // verification of legacy DSA signatures
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
	"time"

	"encoding/asn1"
)

// APK Signing Block ids.
const (
	BlockV2          = 0x7109871a
	BlockV3          = 0xf05368c0
	BlockV31         = 0x1b93ad61
	BlockVerityPad   = 0x42726577
	BlockSourceStamp = 0x6dff800d
	BlockStampV1     = 0x2b09189e
	BlockDependency  = 0x504b4453
	BlockPlayFrost   = 0x2146444e

	attrProofOfRotation = 0x3ba06f8c
	attrStrippingProt   = 0xbeeff00d
	attrRotationMinSDK  = 0x559f8b02

	sigBlockMagic = "APK Sig Block 42"
	chunkSize     = 1 << 20
	eocdSize      = 22
)

// Signature algorithm ids.
const (
	algRSAPSS256    = 0x0101
	algRSAPSS512    = 0x0102
	algRSAPKCS256   = 0x0103
	algRSAPKCS512   = 0x0104
	algECDSA256     = 0x0201
	algECDSA512     = 0x0202
	algDSA256       = 0x0301
	algVerityRSA256 = 0x0421
	algVerityEC256  = 0x0423
	algVerityDSA256 = 0x0425
)

var (
	errNoSigBlock  = errors.New("no APK Signing Block")
	errMalformed   = errors.New("malformed signature data")
	errSigMismatch = errors.New("signature does not verify")
)

func algName(id uint32) string {
	switch id {
	case algRSAPSS256:
		return "RSASSA-PSS with SHA2-256"
	case algRSAPSS512:
		return "RSASSA-PSS with SHA2-512"
	case algRSAPKCS256:
		return "RSASSA-PKCS1-v1_5 with SHA2-256"
	case algRSAPKCS512:
		return "RSASSA-PKCS1-v1_5 with SHA2-512"
	case algECDSA256:
		return "ECDSA with SHA2-256"
	case algECDSA512:
		return "ECDSA with SHA2-512"
	case algDSA256:
		return "DSA with SHA2-256"
	case algVerityRSA256:
		return "RSASSA-PKCS1-v1_5 with SHA2-256 (verity)"
	case algVerityEC256:
		return "ECDSA with SHA2-256 (verity)"
	case algVerityDSA256:
		return "DSA with SHA2-256 (verity)"
	}

	return fmt.Sprintf("unknown 0x%04x", id)
}

// algHash is the content digest hash of a signature algorithm; verity
// algorithms (tree-based digest) report crypto.Hash(0).
func algHash(id uint32) crypto.Hash {
	switch id {
	case algRSAPSS256, algRSAPKCS256, algECDSA256, algDSA256:
		return crypto.SHA256
	case algRSAPSS512, algRSAPKCS512, algECDSA512:
		return crypto.SHA512
	}

	return 0
}

// sigHash is the hash used inside the signature itself.
func sigHash(id uint32) crypto.Hash {
	if h := algHash(id); h != 0 {
		return h
	}

	return crypto.SHA256
}

// zipLayout locates the sections an APK signature covers.
type zipLayout struct {
	size       int64
	cdOffset   int64
	cdSize     int64
	eocdOffset int64
	eocd       []byte
	sigStart   int64 // start of the signing block, == cdOffset when absent
	block      []byte
}

func findLayout(ra io.ReaderAt, size int64) (*zipLayout, error) {
	tail := min(int64(eocdSize+0xffff), size)

	buf := make([]byte, tail)
	if _, err := ra.ReadAt(buf, size-tail); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	for i := len(buf) - eocdSize; i >= 0; i-- {
		if binary.LittleEndian.Uint32(buf[i:]) != 0x06054b50 {
			continue
		}

		commentLen := int(binary.LittleEndian.Uint16(buf[i+20:]))
		if i+eocdSize+commentLen != len(buf) {
			continue
		}

		l := &zipLayout{
			size:       size,
			eocdOffset: size - tail + int64(i),
			eocd:       buf[i:],
			cdSize:     int64(binary.LittleEndian.Uint32(buf[i+12:])),
			cdOffset:   int64(binary.LittleEndian.Uint32(buf[i+16:])),
		}

		if l.cdOffset+l.cdSize > l.eocdOffset {
			return nil, fmt.Errorf("%w: central directory overlaps EOCD", errMalformed)
		}

		l.sigStart = l.cdOffset

		return l, nil
	}

	return nil, fmt.Errorf("%w: end of central directory not found", errMalformed)
}

// readSigningBlock fills l.block with the APK Signing Block, if present.
func (l *zipLayout) readSigningBlock(ra io.ReaderAt) error {
	if l.cdOffset < 32 {
		return errNoSigBlock
	}

	footer := make([]byte, 24)
	if _, err := ra.ReadAt(footer, l.cdOffset-24); err != nil {
		return err
	}

	if string(footer[8:]) != sigBlockMagic {
		return errNoSigBlock
	}

	rawSize := binary.LittleEndian.Uint64(footer)
	if rawSize < 24 || rawSize > 64<<20 || rawSize+8 > uint64(l.cdOffset) {
		return fmt.Errorf("%w: signing block size %d", errMalformed, rawSize)
	}

	blockSize := int64(rawSize)
	start := l.cdOffset - blockSize - 8

	block := make([]byte, blockSize+8)
	if _, err := ra.ReadAt(block, start); err != nil {
		return err
	}

	if binary.LittleEndian.Uint64(block) != rawSize {
		return fmt.Errorf("%w: signing block header and footer sizes differ", errMalformed)
	}

	l.sigStart = start
	l.block = block[8 : len(block)-24]

	return nil
}

// pairs returns the id/value pairs of the signing block.
func (l *zipLayout) pairs() (map[uint32][]byte, []uint32, error) {
	out := map[uint32][]byte{}

	var order []uint32

	b := l.block
	for len(b) > 0 {
		if len(b) < 12 {
			return nil, nil, fmt.Errorf("%w: truncated pair", errMalformed)
		}

		n := binary.LittleEndian.Uint64(b)
		if n < 4 || n > uint64(len(b))-8 {
			return nil, nil, fmt.Errorf("%w: pair length %d", errMalformed, n)
		}

		id := binary.LittleEndian.Uint32(b[8:])
		if _, dup := out[id]; !dup {
			order = append(order, id)
			out[id] = b[12 : 8+n]
		}

		b = b[8+n:]
	}

	return out, order, nil
}

// lp reads one uint32-length-prefixed slice.
func lp(b []byte) ([]byte, []byte, error) {
	if len(b) < 4 {
		return nil, nil, fmt.Errorf("%w: truncated length prefix", errMalformed)
	}

	n := binary.LittleEndian.Uint32(b)
	if uint64(n) > uint64(len(b))-4 {
		return nil, nil, fmt.Errorf("%w: length %d exceeds %d", errMalformed, n, len(b)-4)
	}

	return b[4 : 4+n], b[4+n:], nil
}

func lpSeq(b []byte, fn func([]byte) error) error {
	for len(b) > 0 {
		v, rest, err := lp(b)
		if err != nil {
			return err
		}

		if err := fn(v); err != nil {
			return err
		}

		b = rest
	}

	return nil
}

// contentDigest computes the chunked v2/v3 digest of the three signed sections.
func (l *zipLayout) contentDigest(ra io.ReaderAt, h crypto.Hash) ([]byte, error) {
	eocd := bytes.Clone(l.eocd[:eocdSize])
	binary.LittleEndian.PutUint32(eocd[16:], uint32(l.sigStart)) //nolint:gosec // offset < 4 GiB (no zip64)

	sections := []io.ReaderAt{
		io.NewSectionReader(ra, 0, l.sigStart),
		io.NewSectionReader(ra, l.cdOffset, l.cdSize),
		bytes.NewReader(eocd),
	}
	lengths := []int64{l.sigStart, l.cdSize, eocdSize}

	var (
		chunks [][]byte
		buf    = make([]byte, chunkSize)
	)

	for i, sec := range sections {
		for off := int64(0); off < lengths[i]; off += chunkSize {
			n := min(int64(chunkSize), lengths[i]-off)
			if _, err := sec.ReadAt(buf[:n], off); err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}

			d := h.New()

			var hdr [5]byte

			hdr[0] = 0xa5
			binary.LittleEndian.PutUint32(hdr[1:], uint32(n)) //nolint:gosec // n <= 1 MiB
			d.Write(hdr[:])
			d.Write(buf[:n])
			chunks = append(chunks, d.Sum(nil))
		}
	}

	top := h.New()

	var hdr [5]byte

	hdr[0] = 0x5a
	binary.LittleEndian.PutUint32(hdr[1:], uint32(len(chunks))) //nolint:gosec // bounded by file size / 1 MiB

	top.Write(hdr[:])

	for _, c := range chunks {
		top.Write(c)
	}

	return top.Sum(nil), nil
}

// verifyRaw checks sig over data with pub for a v2/v3 algorithm id.
func verifyRaw(pub any, alg uint32, data, sig []byte) error {
	h := sigHash(alg)
	hh := h.New()
	hh.Write(data)
	sum := hh.Sum(nil)

	switch alg {
	case algRSAPSS256, algRSAPSS512:
		k, ok := pub.(*rsa.PublicKey)
		if !ok {
			return errSigMismatch
		}

		return rsa.VerifyPSS(k, h, sum, sig, &rsa.PSSOptions{SaltLength: h.Size(), Hash: h})
	case algRSAPKCS256, algRSAPKCS512, algVerityRSA256:
		k, ok := pub.(*rsa.PublicKey)
		if !ok {
			return errSigMismatch
		}

		return rsa.VerifyPKCS1v15(k, h, sum, sig)
	case algECDSA256, algECDSA512, algVerityEC256:
		k, ok := pub.(*ecdsa.PublicKey)
		if !ok || !ecdsa.VerifyASN1(k, sum, sig) {
			return errSigMismatch
		}

		return nil
	case algDSA256, algVerityDSA256:
		return verifyDSA(pub, sum, sig)
	}

	return fmt.Errorf("%w: unsupported algorithm 0x%04x", errSigMismatch, alg)
}

func verifyDSA(pub any, sum, sig []byte) error {
	k, ok := pub.(*dsa.PublicKey)
	if !ok {
		return errSigMismatch
	}

	var rs struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(sig, &rs); err != nil {
		return errSigMismatch
	}

	// FIPS 186-3: use the leftmost bits of the hash matching the size of Q.
	if n := (k.Q.BitLen() + 7) / 8; len(sum) > n {
		sum = sum[:n]
	}

	if !dsa.Verify(k, sum, rs.R, rs.S) {
		return errSigMismatch
	}

	return nil
}

// Signer is one signer of one scheme.
type Signer struct {
	Certificate      *CertInfo  `json:"certificate"`
	Chain            []CertInfo `json:"chain,omitempty"`
	MinSDK           int        `json:"min_sdk,omitempty"`
	MaxSDK           int        `json:"max_sdk,omitempty"`
	DigestAlgorithms []string   `json:"digest_algorithms,omitempty"`
	SignatureAlgs    []string   `json:"signature_algorithms,omitempty"`
	Verified         bool       `json:"verified"`
	Errors           []string   `json:"errors,omitempty"`
}

// LineageNode is one certificate of a v3 SigningCertificateLineage.
type LineageNode struct {
	Certificate *CertInfo `json:"certificate"`
	Flags       uint32    `json:"flags"`
	SigAlgo     string    `json:"signature_algorithm,omitempty"`
}

// SchemeResult is the outcome of one signature scheme.
type SchemeResult struct {
	Scheme   string        `json:"scheme"`
	Verified bool          `json:"verified"`
	Signers  []Signer      `json:"signers"`
	Lineage  []LineageNode `json:"lineage,omitempty"`
	Errors   []string      `json:"errors,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
	// RotationMinSDK is the v3.1 rotation-min-sdk-version attribute of v3.
	RotationMinSDK int `json:"rotation_min_sdk,omitempty"`
}

type digestCache struct {
	l    *zipLayout
	ra   io.ReaderAt
	sums map[crypto.Hash][]byte
	errs map[crypto.Hash]error
}

func (c *digestCache) get(h crypto.Hash) ([]byte, error) {
	if s, ok := c.sums[h]; ok {
		return s, c.errs[h]
	}

	s, err := c.l.contentDigest(c.ra, h)
	c.sums[h], c.errs[h] = s, err

	return s, err
}

// parseV2V3 parses and verifies a v2, v3 or v3.1 block value.
func parseV2V3(scheme string, val []byte, dc *digestCache, now time.Time) SchemeResult {
	res := SchemeResult{Scheme: scheme}
	v3 := scheme != "v2"

	signers, _, err := lp(val)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
		return res
	}

	err = lpSeq(signers, func(sb []byte) error {
		s, lineage, rotMin := parseSigner(sb, v3, dc, now)
		res.Signers = append(res.Signers, s)

		if len(lineage) > 0 {
			res.Lineage = lineage
		}

		if rotMin > 0 {
			res.RotationMinSDK = rotMin
		}

		return nil
	})
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}

	if len(res.Signers) == 0 {
		res.Errors = append(res.Errors, "no signers")
	}

	res.Verified = len(res.Errors) == 0 && len(res.Signers) > 0

	for _, s := range res.Signers {
		res.Verified = res.Verified && s.Verified
	}

	return res
}

type rawSigner struct {
	signedData []byte
	sigs       []byte
	pubKey     []byte
	minSDK     uint32
	maxSDK     uint32
}

func splitSigner(sb []byte, v3 bool) (rawSigner, error) {
	var (
		rs  rawSigner
		err error
	)

	if rs.signedData, sb, err = lp(sb); err != nil {
		return rs, err
	}

	if v3 {
		if len(sb) < 8 {
			return rs, fmt.Errorf("%w: truncated v3 signer", errMalformed)
		}

		rs.minSDK, rs.maxSDK = binary.LittleEndian.Uint32(sb), binary.LittleEndian.Uint32(sb[4:])
		sb = sb[8:]
	}

	if rs.sigs, sb, err = lp(sb); err != nil {
		return rs, err
	}

	if rs.pubKey, _, err = lp(sb); err != nil {
		return rs, err
	}

	return rs, nil
}

//nolint:cyclop,gocyclo // linear walk of the signer structure, each step can fail
func parseSigner(sb []byte, v3 bool, dc *digestCache, now time.Time) (Signer, []LineageNode, int) {
	var (
		s       Signer
		lineage []LineageNode
		rotMin  int
	)

	fail := func(err error) (Signer, []LineageNode, int) {
		s.Errors = append(s.Errors, err.Error())
		return s, lineage, rotMin
	}

	rs, err := splitSigner(sb, v3)
	if err != nil {
		return fail(err)
	}

	s.MinSDK, s.MaxSDK = int(rs.minSDK), int(rs.maxSDK)

	pub, err := x509.ParsePKIXPublicKey(rs.pubKey)
	if err != nil {
		return fail(fmt.Errorf("public key: %w", err))
	}

	// Signatures over signed data.
	sigAlgs := map[uint32]bool{}

	err = lpSeq(rs.sigs, func(b []byte) error {
		if len(b) < 4 {
			return errMalformed
		}

		alg := binary.LittleEndian.Uint32(b)

		sig, _, err := lp(b[4:])
		if err != nil {
			return err
		}

		s.SignatureAlgs = append(s.SignatureAlgs, algName(alg))
		sigAlgs[alg] = true

		if err := verifyRaw(pub, alg, rs.signedData, sig); err != nil {
			return fmt.Errorf("%s: %w", algName(alg), errSigMismatch)
		}

		return nil
	})
	if err != nil {
		return fail(err)
	}

	if len(sigAlgs) == 0 {
		return fail(fmt.Errorf("%w: no signatures", errMalformed))
	}

	// Signed data: digests, certificates, [v3: min/max sdk], attributes.
	digests, rest, err := lp(rs.signedData)
	if err != nil {
		return fail(err)
	}

	certs, rest, err := lp(rest)
	if err != nil {
		return fail(err)
	}

	if v3 {
		if len(rest) < 8 {
			return fail(fmt.Errorf("%w: truncated v3 signed data", errMalformed))
		}

		if binary.LittleEndian.Uint32(rest) != rs.minSDK || binary.LittleEndian.Uint32(rest[4:]) != rs.maxSDK {
			s.Errors = append(s.Errors, "min/max SDK in signed data differ from the signer record")
		}

		rest = rest[8:]
	}

	attrs, _, err := lp(rest)
	if err != nil {
		return fail(err)
	}

	if err := s.readCerts(certs, pub, now); err != nil {
		return fail(err)
	}

	if err := s.checkDigests(digests, sigAlgs, dc); err != nil {
		s.Errors = append(s.Errors, err.Error())
	}

	_ = lpSeq(attrs, func(a []byte) error {
		if len(a) < 4 {
			return nil
		}

		switch binary.LittleEndian.Uint32(a) {
		case attrProofOfRotation:
			l, err := parseLineage(a[4:], now)
			if err != nil {
				s.Errors = append(s.Errors, "lineage: "+err.Error())
			}

			lineage = l
		case attrRotationMinSDK:
			if len(a) >= 8 {
				rotMin = int(binary.LittleEndian.Uint32(a[4:]))
			}
		}

		return nil
	})

	s.Verified = len(s.Errors) == 0

	return s, lineage, rotMin
}

func (s *Signer) readCerts(certs []byte, pub any, now time.Time) error {
	err := lpSeq(certs, func(der []byte) error {
		ci, err := CertInfoFromDER(der, now)
		if err != nil {
			return fmt.Errorf("certificate: %w", err)
		}

		if s.Certificate == nil {
			s.Certificate = ci
		} else {
			s.Chain = append(s.Chain, *ci)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if s.Certificate == nil {
		return fmt.Errorf("%w: no certificate", errMalformed)
	}

	if !publicKeyEqual(s.Certificate.cert.PublicKey, pub) {
		return fmt.Errorf("%w: certificate public key differs from the signer's", errMalformed)
	}

	return nil
}

func publicKeyEqual(a, b any) bool {
	type equaler interface{ Equal(crypto.PublicKey) bool }

	if e, ok := a.(equaler); ok {
		return e.Equal(b)
	}

	if ka, ok := a.(*dsa.PublicKey); ok {
		kb, ok := b.(*dsa.PublicKey)
		return ok && ka.Y.Cmp(kb.Y) == 0 && ka.P.Cmp(kb.P) == 0
	}

	return false
}

func (s *Signer) checkDigests(digests []byte, sigAlgs map[uint32]bool, dc *digestCache) error {
	seen := map[uint32]bool{}

	err := lpSeq(digests, func(b []byte) error {
		if len(b) < 4 {
			return errMalformed
		}

		alg := binary.LittleEndian.Uint32(b)

		want, _, err := lp(b[4:])
		if err != nil {
			return err
		}

		seen[alg] = true
		s.DigestAlgorithms = append(s.DigestAlgorithms, algName(alg))

		h := algHash(alg)
		if h == 0 {
			return nil // verity digest: not recomputed
		}

		got, err := dc.get(h)
		if err != nil {
			return fmt.Errorf("content digest: %w", err)
		}

		if !bytes.Equal(got, want) {
			return fmt.Errorf("%s content digest does not match the file: %w", algName(alg), errSigMismatch)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for alg := range sigAlgs {
		if !seen[alg] {
			return fmt.Errorf("%w: signature algorithm %s has no digest", errMalformed, algName(alg))
		}
	}

	return nil
}

// parseLineage decodes a v3 proof-of-rotation attribute and checks each
// certificate signs the next.
func parseLineage(b []byte, now time.Time) ([]LineageNode, error) {
	if len(b) < 4 {
		return nil, errMalformed
	}

	var (
		out      []LineageNode
		prevCert *CertInfo
	)

	err := lpSeq(b[4:], func(node []byte) error {
		signed, rest, err := lp(node)
		if err != nil {
			return err
		}

		if len(rest) < 8 {
			return errMalformed
		}

		flags := binary.LittleEndian.Uint32(rest)

		sig, _, err := lp(rest[8:])
		if err != nil {
			return err
		}

		// Signed data: this certificate and the algorithm the previous
		// certificate used to sign it.
		der, sd, err := lp(signed)
		if err != nil {
			return err
		}

		ci, err := CertInfoFromDER(der, now)
		if err != nil {
			return err
		}

		n := LineageNode{Certificate: ci, Flags: flags}

		if prevCert != nil {
			if len(sd) < 4 {
				return errMalformed
			}

			alg := binary.LittleEndian.Uint32(sd)
			n.SigAlgo = algName(alg)

			if err := verifyRaw(prevCert.cert.PublicKey, alg, signed, sig); err != nil {
				return fmt.Errorf("lineage link to %s: %w", ci.SHA256[:12], err)
			}
		}

		out = append(out, n)
		prevCert = ci

		return nil
	})

	return out, err
}

// hashFor returns a new hash for a JAR digest algorithm name.
func hashFor(name string) hash.Hash {
	switch name {
	case "SHA-256", "SHA256":
		return sha256.New()
	case "SHA-512", "SHA512":
		return sha512.New()
	case "SHA-384", "SHA384":
		return sha512.New384()
	case "SHA1", "SHA-1":
		return crypto.SHA1.New()
	}

	return nil
}

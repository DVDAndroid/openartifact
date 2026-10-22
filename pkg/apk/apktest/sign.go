// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apktest

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// Key is a signing key with its self-signed certificate.
type Key struct {
	Signer crypto.Signer
	Cert   []byte // DER
}

// SHA256 returns the certificate fingerprint as apksigner prints it.
func (k *Key) SHA256() string {
	s := sha256.Sum256(k.Cert)
	return hex.EncodeToString(s[:])
}

// NewRSAKey generates a 2048-bit RSA key and certificate with the given CN.
func NewRSAKey(cn string) *Key {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	return selfSign(k, cn)
}

// NewECKey generates a P-256 key and certificate with the given CN.
func NewECKey(cn string) *Key {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	return selfSign(k, cn)
}

func selfSign(k crypto.Signer, cn string) *Key {
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"OpenArtifact Test"}},
		NotBefore:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, k.Public(), k)
	if err != nil {
		panic(err)
	}

	return &Key{Signer: k, Cert: der}
}

// Sign selects the schemes Build signs with.
type Sign struct {
	Key *Key
	V1  bool
	V2  bool
	V3  bool
}

// Build returns the APK bytes.
func (b *Builder) Build(s Sign) []byte {
	entries := b.entries()

	if s.Key != nil && s.V1 {
		entries = append(entries, v1Files(entries, s)...)
	}

	z := writeZip(entries)

	if s.Key == nil || (!s.V2 && !s.V3) {
		return z
	}

	return addSigningBlock(z, s)
}

func (k *Key) isRSA() bool {
	_, ok := k.Signer.(*rsa.PrivateKey)
	return ok
}

func v1Files(entries []entry, s Sign) []entry {
	var mf bytes.Buffer

	mf.WriteString("Manifest-Version: 1.0\r\nCreated-By: apktest\r\n\r\n")

	sections := map[string][]byte{}

	for _, e := range entries {
		d := sha256.Sum256(e.data)
		sec := fmt.Sprintf("Name: %s\r\nSHA-256-Digest: %s\r\n\r\n", e.name, base64.StdEncoding.EncodeToString(d[:]))
		sections[e.name] = []byte(sec)
		mf.WriteString(sec)
	}

	var sf bytes.Buffer

	md := sha256.Sum256(mf.Bytes())
	sf.WriteString("Signature-Version: 1.0\r\nCreated-By: apktest\r\n")
	sf.WriteString("SHA-256-Digest-Manifest: " + base64.StdEncoding.EncodeToString(md[:]) + "\r\n")

	var schemes []string
	if s.V2 {
		schemes = append(schemes, "2")
	}

	if s.V3 {
		schemes = append(schemes, "3")
	}

	if len(schemes) > 0 {
		sf.WriteString("X-Android-APK-Signed: " + strings.Join(schemes, ", ") + "\r\n")
	}

	sf.WriteString("\r\n")

	for _, e := range entries {
		d := sha256.Sum256(sections[e.name])
		fmt.Fprintf(&sf, "Name: %s\r\nSHA-256-Digest: %s\r\n\r\n", e.name, base64.StdEncoding.EncodeToString(d[:]))
	}

	ext := ".EC"
	if s.Key.isRSA() {
		ext = ".RSA"
	}

	return []entry{
		{name: "META-INF/MANIFEST.MF", data: mf.Bytes()},
		{name: "META-INF/CERT.SF", data: sf.Bytes()},
		{name: "META-INF/CERT" + ext, data: pkcs7(s.Key, sf.Bytes())},
	}
}

type algID struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type issuerSerial struct {
	Issuer asn1.RawValue
	Serial *big.Int
}

type signerInfo struct {
	Version         int
	SID             issuerSerial
	DigestAlgorithm algID
	EncAlgorithm    algID
	Signature       []byte
}

type signedData struct {
	Version          int
	DigestAlgorithms []algID `asn1:"set"`
	ContentInfo      struct{ ContentType asn1.ObjectIdentifier }
	Certificates     asn1.RawValue
	SignerInfos      []signerInfo `asn1:"set"`
}

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue
}

func pkcs7(k *Key, content []byte) []byte {
	cert, err := x509.ParseCertificate(k.Cert)
	if err != nil {
		panic(err)
	}

	sha256OID := asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	null := asn1.RawValue{Tag: asn1.TagNull}
	enc := algID{Algorithm: asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}} // ecdsa-with-SHA256

	if k.isRSA() {
		enc = algID{Algorithm: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}, Parameters: null}
	}

	d := sha256.Sum256(content)

	sig, err := k.Signer.Sign(rand.Reader, d[:], crypto.SHA256)
	if err != nil {
		panic(err)
	}

	sd := signedData{
		Version:          1,
		DigestAlgorithms: []algID{{Algorithm: sha256OID, Parameters: null}},
		Certificates:     asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: k.Cert},
		SignerInfos: []signerInfo{{
			Version:         1,
			SID:             issuerSerial{Issuer: asn1.RawValue{FullBytes: cert.RawIssuer}, Serial: cert.SerialNumber},
			DigestAlgorithm: algID{Algorithm: sha256OID, Parameters: null},
			EncAlgorithm:    enc,
			Signature:       sig,
		}},
	}
	sd.ContentInfo.ContentType = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}

	inner, err := asn1.Marshal(sd)
	if err != nil {
		panic(err)
	}

	out, err := asn1.Marshal(contentInfo{
		ContentType: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2},
		Content:     asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: inner},
	})
	if err != nil {
		panic(err)
	}

	return out
}

func lpBytes(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = binary.LittleEndian.AppendUint32(out, uint32(len(p))) //nolint:gosec // test data
		out = append(out, p...)
	}

	return out
}

func concat(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func u32(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

// addSigningBlock inserts a v2 and/or v3 APK Signing Block into zip z.
func addSigningBlock(z []byte, s Sign) []byte {
	eocd := bytes.LastIndex(z, []byte{0x50, 0x4b, 0x05, 0x06})
	cdOff := int(binary.LittleEndian.Uint32(z[eocd+16:]))

	digest := chunkedDigest(z[:cdOff], z[cdOff:eocd], z[eocd:], cdOff)

	alg := uint32(0x0201)
	if s.Key.isRSA() {
		alg = 0x0103
	}

	pub, err := x509.MarshalPKIXPublicKey(s.Key.Signer.Public())
	if err != nil {
		panic(err)
	}

	sign := func(data []byte) []byte {
		d := sha256.Sum256(data)

		sig, err := s.Key.Signer.Sign(rand.Reader, d[:], crypto.SHA256)
		if err != nil {
			panic(err)
		}

		return sig
	}

	digests := lpBytes(concat(u32(alg), lpBytes(digest)))
	certs := lpBytes(s.Key.Cert)

	var pairs []byte

	addPair := func(id uint32, val []byte) {
		pairs = binary.LittleEndian.AppendUint64(pairs, uint64(len(val)+4))
		pairs = append(pairs, u32(id)...)
		pairs = append(pairs, val...)
	}

	if s.V2 {
		sd := concat(lpBytes(digests), lpBytes(certs), lpBytes(nil))
		signer := concat(lpBytes(sd), lpBytes(lpBytes(concat(u32(alg), lpBytes(sign(sd))))), lpBytes(pub))
		addPair(0x7109871a, lpBytes(lpBytes(signer)))
	}

	if s.V3 {
		minMax := concat(u32(24), u32(0x7fffffff))
		sd := concat(lpBytes(digests), lpBytes(certs), minMax, lpBytes(nil))
		signer := concat(lpBytes(sd), minMax, lpBytes(lpBytes(concat(u32(alg), lpBytes(sign(sd))))), lpBytes(pub))
		addPair(0xf05368c0, lpBytes(lpBytes(signer)))
	}

	size := uint64(len(pairs) + 8 + 16)
	block := binary.LittleEndian.AppendUint64(nil, size)
	block = append(block, pairs...)
	block = binary.LittleEndian.AppendUint64(block, size)
	block = append(block, "APK Sig Block 42"...)

	out := concat(z[:cdOff], block, z[cdOff:])
	newEOCD := len(out) - (len(z) - eocd)
	binary.LittleEndian.PutUint32(out[newEOCD+16:], uint32(cdOff+len(block))) //nolint:gosec // small test archive

	return out
}

func chunkedDigest(entries, cd, eocd []byte, sigStart int) []byte {
	e := bytes.Clone(eocd)
	binary.LittleEndian.PutUint32(e[16:], uint32(sigStart)) //nolint:gosec // small test archive

	var chunks [][]byte

	for _, sec := range [][]byte{entries, cd, e} {
		for off := 0; off < len(sec); off += 1 << 20 {
			end := min(off+1<<20, len(sec))
			h := sha256.New()
			h.Write([]byte{0xa5})
			h.Write(u32(uint32(end - off))) //nolint:gosec // <= 1 MiB
			h.Write(sec[off:end])
			chunks = append(chunks, h.Sum(nil))
		}
	}

	h := sha256.New()
	h.Write([]byte{0x5a})
	h.Write(u32(uint32(len(chunks)))) //nolint:gosec // small

	for _, c := range chunks {
		h.Write(c)
	}

	return h.Sum(nil)
}

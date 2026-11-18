// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCert struct {
	der  []byte
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// newTestCert creates a certificate for cn signed by parent (self-signed when parent is nil).
func newTestCert(t *testing.T, cn string, isCA bool, parent *testCert) *testCert {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  isCA,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	signerCert, signerKey := tmpl, key
	if parent != nil {
		signerCert, signerKey = parent.cert, parent.key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, &key.PublicKey, signerKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return &testCert{der: der, cert: cert, key: key}
}

func (c *testCert) pem() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.der})
}

func TestCertType(t *testing.T) {
	t.Parallel()

	root := newTestCert(t, "root", true, nil)
	intermediate := newTestCert(t, "intermediate", true, root)
	leaf := newTestCert(t, "leaf", false, intermediate)

	assert.Equal(t, CertTypeRoot, CertType(root.cert))
	assert.Equal(t, CertTypeIntermediate, CertType(intermediate.cert))
	assert.Empty(t, CertType(leaf.cert), "leaf is not a CA")
}

func TestParseIntermediateCerts(t *testing.T) {
	t.Parallel()

	root := newTestCert(t, "root", true, nil)
	intermediate := newTestCert(t, "intermediate", true, root)
	leaf := newTestCert(t, "leaf", false, intermediate)

	bundle := append(append(append([]byte{}, root.pem()...), intermediate.pem()...), leaf.pem()...)
	withKey := append(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("secret")}), bundle...)

	cases := []struct {
		name         string
		data         []byte
		includeRoots bool
		want         []string
	}{
		{name: "bundle keeps only intermediates", data: bundle, want: []string{"CN=intermediate"}},
		{name: "bundle with roots", data: bundle, includeRoots: true, want: []string{"CN=root", "CN=intermediate"}},
		{name: "private keys are skipped", data: withKey, want: []string{"CN=intermediate"}},
		{name: "single DER certificate", data: intermediate.der, want: []string{"CN=intermediate"}},
		{name: "root only", data: root.pem(), want: nil},
		{name: "root only with roots", data: root.pem(), includeRoots: true, want: []string{"CN=root"}},
		{name: "garbage", data: []byte("not a certificate"), want: nil},
		{name: "empty", data: nil, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var subjects []string
			for _, c := range ParseIntermediateCerts(tc.data, "src", tc.includeRoots) {
				subjects = append(subjects, c.Subject)
				assert.Equal(t, "src", c.Source)
				assert.Equal(t, "CN=root", c.Issuer)
				assert.Equal(t, map[string]string{"CN=root": CertTypeRoot, "CN=intermediate": CertTypeIntermediate}[c.Subject], c.Type)
				assert.Len(t, c.Fingerprint, 64)
				assert.Equal(t, "ECDSA-P-256", c.PublicKey)
				assert.False(t, c.NameConstrained)
			}

			assert.Equal(t, tc.want, subjects)
		})
	}
}

func TestNameConstrainedAndKeyDescription(t *testing.T) {
	t.Parallel()

	root := newTestCert(t, "root", true, nil)
	assert.False(t, isNameConstrained(root.cert))

	constrained := *root.cert
	constrained.PermittedDNSDomains = []string{"example.com"}
	assert.True(t, isNameConstrained(&constrained))

	assert.Equal(t, "ECDSA-P-256", describePublicKey(root.cert))

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	assert.Equal(t, "RSA-2048", describePublicKey(&x509.Certificate{PublicKey: &rsaKey.PublicKey}))

	edKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	assert.Equal(t, "Ed25519", describePublicKey(&x509.Certificate{PublicKey: edKey}))
}

func TestDedupeIntermediateCerts(t *testing.T) {
	t.Parallel()

	got := DedupeIntermediateCerts([]IntermediateCert{
		{Fingerprint: "b", Source: "/z"},
		{Fingerprint: "a", Source: "/y"},
		{Fingerprint: "b", Source: "/a"},
	})

	assert.Equal(t, []IntermediateCert{
		{Fingerprint: "a", Source: "/y"},
		{Fingerprint: "b", Source: "/a"},
	}, got)
}

func TestIsCertFileName(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{
		"ca-certificates.crt": true,
		"Foo.PEM":             true,
		"5ad8a5d6.0":          true,
		"README":              false,
		"key.key":             false,
		"noext":               false,
	} {
		assert.Equal(t, want, IsCertFileName(name), name)
	}
}

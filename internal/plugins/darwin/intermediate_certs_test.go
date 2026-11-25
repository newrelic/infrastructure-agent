// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package darwin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPEMCert(t *testing.T, cn string, isCA bool, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
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
	}

	if parent == nil {
		parent, parentKey = tmpl, key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestCollectIntermediateCerts(t *testing.T) {
	t.Parallel()

	root, rootKey, rootPEM := newPEMCert(t, "root", true, nil, nil)
	inter, interKey, interPEM := newPEMCert(t, "intermediate", true, root, rootKey)
	_, _, leafPEM := newPEMCert(t, "leaf", false, inter, interKey)

	read := func(keychain string) ([]byte, error) {
		switch keychain {
		case "a.keychain":
			return append(append(append([]byte{}, rootPEM...), interPEM...), leafPEM...), nil
		case "b.keychain":
			return interPEM, nil // duplicate of the one in a.keychain
		}

		return nil, errors.New("locked keychain")
	}

	keychains := []string{"a.keychain", "b.keychain", "c.keychain"}

	withRoots, err := collectIntermediateCerts(keychains, read, true)
	require.Error(t, err)
	assert.Len(t, withRoots, 2, "root and intermediate")

	certs, err := collectIntermediateCerts(keychains, read, false)

	require.Error(t, err, "a failing keychain is reported")
	require.Len(t, certs, 1, "partial results are kept and deduplicated")
	assert.Equal(t, "CN=intermediate", certs[0].Subject)
	assert.Equal(t, "a.keychain", certs[0].Source)
}

// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package linux

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pemCert(t *testing.T, cn string, isCA bool, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
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

	root, rootKey, rootPEM := pemCert(t, "root", true, nil, nil)
	_, _, interPEM := pemCert(t, "intermediate", true, root, rootKey)

	dir := t.TempDir()
	other := t.TempDir()
	anchors := filepath.Join(dir, "anchors")
	require.NoError(t, os.Mkdir(anchors, 0o755))

	// Same intermediate in a bundle and an anchor file, plus a root and an unrelated file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundle.crt"), append(append([]byte{}, rootPEM...), interPEM...), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(anchors, "inter.pem"), interPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), interPEM, 0o600))
	require.NoError(t, os.Symlink(filepath.Join(anchors, "inter.pem"), filepath.Join(dir, "abcd1234.0")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "missing"), filepath.Join(dir, "broken.0")))

	dirs := []string{dir, filepath.Join(other, "does-not-exist")}

	certs, err := collectIntermediateCerts(dirs, false)

	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, "CN=intermediate", certs[0].Subject)
	assert.Equal(t, "CN=root", certs[0].Issuer)
	assert.Equal(t, "intermediate", certs[0].Type)

	certs, err = collectIntermediateCerts(dirs, true)

	require.NoError(t, err)
	require.Len(t, certs, 2)
	assert.ElementsMatch(t, []string{"root", "intermediate"}, []string{certs[0].Type, certs[1].Type})
}

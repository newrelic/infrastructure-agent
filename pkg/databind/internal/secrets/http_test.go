// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	. "net/http"
	"net/http/httptest"
	"os"
	"path"
	"regexp"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

type certs struct {
	CACert		string
	CAKey		string
	ServerCert	string
	ServerKey	string
	ClientCert	string
	ClientKey	string
}

var certPaths	certs

func TestClientCertHttpRequest(t *testing.T) {
	ts, err := newLocalHttpsTestServer(
		`{ "message": "Hello, world" }`, 200)
	if err != nil {
		t.Errorf("failed to start TLS servr: %s", err)
	}

	defer ts.Close()
	HTTP := &http{
		URL:     ts.URL,
		TLSConfig: (tlsConfig{
			Enable: true,
			InsecureSkipVerify: false,
			Ca: certPaths.CACert,
			ClientCert: certPaths.ClientCert,
			ClientKey: certPaths.ClientKey,
		}),
		Headers: make(map[string]string),
	}

	dt, err := httpRequest(HTTP, "GET", nil)
	if err != nil {
		t.Errorf("call failed: %s", err)
	}

	smap := data.InterfaceMap{}
	if err := json.Unmarshal(dt, &smap); err != nil {
		t.Errorf("unable to decode response: %s", err)
	}

	if smap["message"] != "Hello, world" {
		t.Errorf("expected message, got %v", smap)
	}
}

func TestClientCertHttpRequestFails(t *testing.T) {
	ts, err := newLocalHttpsTestServer(
		`{ "message": "Hello, world" }`, 200)
	if err != nil {
		t.Errorf("failed to start TLS servr: %s", err)
	}

	defer ts.Close()
	HTTP := &http{
		URL:     ts.URL,
		TLSConfig: (tlsConfig{
			Enable: true,
			InsecureSkipVerify: false,
			Ca: certPaths.CACert,
		}),
		Headers: make(map[string]string),
	}

	_, err = httpRequest(HTTP, "GET", nil)
	if err == nil {
		t.Errorf("call should have returned error but did not")
	}

	match, _ := regexp.MatchString("remote error: tls:", err.Error())
	if !match {
		t.Errorf("error did not match expected pattern: %s", err.Error())
	}
}

func newLocalHttpsTestServer(response string, rc int) (*httptest.Server, error) {
	ts := httptest.NewUnstartedServer(HandlerFunc(func(w ResponseWriter, r *Request) {
		w.WriteHeader(rc)
		w.Write([]byte(response))
	}))

    cert, err := tls.LoadX509KeyPair(certPaths.ServerCert, certPaths.ServerKey)
    if err != nil {
        return nil, err
    }

	certpool := x509.NewCertPool()
	
	pem, err := os.ReadFile(certPaths.CACert)
	if err != nil {
		return nil, fmt.Errorf("Failed to read certificate authority: %v", err)
	}

	if !certpool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("Can't parse certificate authority")
	}

    ts.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certpool,
	}

    ts.StartTLS()
    return ts, nil
}

func TestMain(m *testing.M) {
	err := setup()
	if err != nil {
		os.Exit(-1)
	}

    exitVal := m.Run()

    teardown()
    
	os.Exit(exitVal)
}

func setup() error {
	err := makeCerts()
	if err != nil {
		return err
	}

	return nil
}

func teardown() {
	deleteCerts()
}

func makeCerts() error {
	certPaths = certs{}

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"New Relic, Inc."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"188 Spear St"},
			PostalCode:    []string{"94015"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	filePath := path.Join(os.TempDir(), "ca.pem")
	err = os.WriteFile(filePath, caPEM.Bytes(), 0666)
	if err != nil {
		return err
	}
	certPaths.CACert = filePath

	filePath = path.Join(os.TempDir(), "ca.key")
	err = os.WriteFile(filePath, caPrivKeyPEM.Bytes(), 0666)
	if err != nil {
		return err
	}
	certPaths.CAKey = filePath

	serverCert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization:  []string{"New Relic, Inc."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"188 Spear St"},
			PostalCode:    []string{"94015"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		IsCA:         false,
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	serverCertPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverCert, ca, &serverCertPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	serverCertPEM := new(bytes.Buffer)
	pem.Encode(serverCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertBytes,
	})

	serverCertPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(serverCertPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverCertPrivKey),
	})

	filePath = path.Join(os.TempDir(), "server.pem")
	err = os.WriteFile(filePath, serverCertPEM.Bytes(), 0666)
	if err != nil {
		return err
	}
	certPaths.ServerCert = filePath

	filePath = path.Join(os.TempDir(), "server.key")
	err = os.WriteFile(filePath, serverCertPrivKeyPEM.Bytes(), 0666)
	if err != nil {
		return err
	}
	certPaths.ServerKey = filePath

	clientCert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization:  []string{"New Relic, Inc."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"188 Spear St"},
			PostalCode:    []string{"94015"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		IsCA:         false,
		SubjectKeyId: []byte{1, 2, 3, 4, 8},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	clientCertPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientCert, ca, &clientCertPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	clientCertPEM := new(bytes.Buffer)
	pem.Encode(clientCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertBytes,
	})

	clientCertPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(clientCertPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientCertPrivKey),
	})

	filePath = path.Join(os.TempDir(), "client.pem")
	err = os.WriteFile(filePath, clientCertPEM.Bytes(), 0666)
	if err != nil {
		return err
	}
	certPaths.ClientCert = filePath

	filePath = path.Join(os.TempDir(), "client.key")
	err = os.WriteFile(filePath, clientCertPrivKeyPEM.Bytes(), 0666)
	if err != nil {
		return err
	}
	certPaths.ClientKey = filePath

	return nil
}

func deleteCerts() {
	if certPaths.CACert != "" {
		_ = os.Remove(certPaths.CACert)
	}
	if certPaths.CAKey != "" {
		_ = os.Remove(certPaths.CAKey)
	}
	if certPaths.ServerCert != "" {
		_ = os.Remove(certPaths.ServerCert)
	}
	if certPaths.ServerKey != "" {
		_ = os.Remove(certPaths.ServerKey)
	}
	if certPaths.ClientCert != "" {
		_ = os.Remove(certPaths.ClientCert)
	}
	if certPaths.ClientKey != "" {
		_ = os.Remove(certPaths.ClientKey)
	}
}
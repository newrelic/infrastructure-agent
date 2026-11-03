// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

// IntermediateCertsPluginID is the inventory identifier of the intermediate certificates plugin.
//
//nolint:gochecknoglobals
var IntermediateCertsPluginID = ids.PluginID{Category: "security", Term: "intermediate_certs"}

var iclog = log.WithComponent("IntermediateCertsPlugin") //nolint:gochecknoglobals

// IntermediateCert is the minimal set of public attributes of a CA certificate (an intermediate, or a root when
// enabled) needed to recognize an unexpected or weak CA: what it is, who issued it, how long it is valid, how
// strong it is, how much it can sign for and where it was found. The raw certificate, the serial number and any
// private key material are never reported.
type IntermediateCert struct {
	// Fingerprint is the SHA-256 of the DER encoding, as lowercase hex. It uniquely identifies the certificate.
	Fingerprint string `json:"id"`
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	// Type is CertTypeIntermediate or CertTypeRoot.
	Type               string `json:"type"`
	NotBefore          string `json:"not_before"`
	NotAfter           string `json:"not_after"`
	SignatureAlgorithm string `json:"signature_algorithm"`
	// PublicKey is the key algorithm and strength, e.g. "RSA-2048", "ECDSA-P-256" or "Ed25519".
	PublicKey string `json:"public_key"`
	// NameConstrained is true when the CA is restricted to some names. An intermediate without constraints can issue
	// valid certificates for any domain, which makes it the dangerous kind to have unexpectedly.
	NameConstrained bool `json:"name_constrained"`
	// Source is the file path (Linux), keychain (macOS) or certificate store (Windows) the certificate was found in.
	Source string `json:"source"`
}

// SortKey implements types.Sortable.
func (c IntermediateCert) SortKey() string {
	return c.Fingerprint
}

// Certificate types reported in IntermediateCert.Type.
const (
	CertTypeIntermediate = "intermediate"
	CertTypeRoot         = "root"
)

// IntermediateCertsCollector returns every intermediate certificate found on the host, plus the root certificates
// when includeRoots is set. It may return partial results together with an error.
type IntermediateCertsCollector func(includeRoots bool) ([]IntermediateCert, error)

// CertType classifies a certificate as CertTypeRoot (a self-signed CA), CertTypeIntermediate (a CA that chains up
// to a different issuer) or "" when it is not a CA at all.
func CertType(cert *x509.Certificate) string {
	if !cert.IsCA {
		return ""
	}

	// Self-signed (root) certificates have identical subject and issuer and are verified by their own key. The
	// signature check protects against a cross-signed intermediate that reuses the subject of a root.
	if bytes.Equal(cert.RawSubject, cert.RawIssuer) && cert.CheckSignatureFrom(cert) == nil {
		return CertTypeRoot
	}

	return CertTypeIntermediate
}

// NewCACert builds the reported metadata for cert if it must be reported: always for intermediates, and for
// roots only when includeRoots is set. The boolean is false when the certificate must be skipped.
func NewCACert(cert *x509.Certificate, source string, includeRoots bool) (IntermediateCert, bool) {
	certType := CertType(cert)
	if certType == "" || (certType == CertTypeRoot && !includeRoots) {
		return IntermediateCert{}, false
	}

	return NewIntermediateCert(cert, source, certType), true
}

// NewIntermediateCert builds the reported metadata from a parsed certificate.
func NewIntermediateCert(cert *x509.Certificate, source, certType string) IntermediateCert {
	sum256 := sha256.Sum256(cert.Raw)

	return IntermediateCert{
		Fingerprint:        hex.EncodeToString(sum256[:]),
		Subject:            cert.Subject.String(),
		Issuer:             cert.Issuer.String(),
		Type:               certType,
		NotBefore:          cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:           cert.NotAfter.UTC().Format(time.RFC3339),
		SignatureAlgorithm: cert.SignatureAlgorithm.String(),
		PublicKey:          describePublicKey(cert),
		NameConstrained:    isNameConstrained(cert),
		Source:             source,
	}
}

// describePublicKey returns the key algorithm with its size or curve, e.g. "RSA-2048".
func describePublicKey(cert *x509.Certificate) string {
	switch key := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA-%d", key.N.BitLen())
	case *ecdsa.PublicKey:
		return "ECDSA-" + key.Curve.Params().Name
	case ed25519.PublicKey:
		return "Ed25519"
	default:
		return cert.PublicKeyAlgorithm.String()
	}
}

// isNameConstrained reports whether the certificate carries any X.509 name constraint.
func isNameConstrained(cert *x509.Certificate) bool {
	return len(cert.PermittedDNSDomains) > 0 || len(cert.ExcludedDNSDomains) > 0 ||
		len(cert.PermittedIPRanges) > 0 || len(cert.ExcludedIPRanges) > 0 ||
		len(cert.PermittedEmailAddresses) > 0 || len(cert.ExcludedEmailAddresses) > 0 ||
		len(cert.PermittedURIDomains) > 0 || len(cert.ExcludedURIDomains) > 0
}

// ParseIntermediateCerts extracts the intermediate certificates (and the roots, if includeRoots) from data, which can be a PEM file or bundle
// (any non-CERTIFICATE block, such as private keys, is skipped without being parsed) or a single DER certificate.
// Certificates that can't be parsed are skipped.
func ParseIntermediateCerts(data []byte, source string, includeRoots bool) []IntermediateCert {
	var result []IntermediateCert

	add := func(der []byte) {
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			iclog.WithField("source", source).WithError(err).Debug("Skipping unparsable certificate.")

			return
		}

		if c, ok := NewCACert(cert, source, includeRoots); ok {
			result = append(result, c)
		}
	}

	foundPEM := false
	rest := data

	for {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		foundPEM = true

		if block.Type == "CERTIFICATE" || block.Type == "TRUSTED CERTIFICATE" {
			add(block.Bytes)
		}
	}

	if !foundPEM {
		add(data)
	}

	return result
}

// DedupeIntermediateCerts removes duplicates by fingerprint (the same certificate usually appears in several
// directories and bundles) and sorts the result. The lexicographically smallest source wins so output is stable.
func DedupeIntermediateCerts(certs []IntermediateCert) []IntermediateCert {
	byFingerprint := make(map[string]IntermediateCert, len(certs))

	for _, c := range certs {
		if existing, ok := byFingerprint[c.Fingerprint]; !ok || c.Source < existing.Source {
			byFingerprint[c.Fingerprint] = c
		}
	}

	result := make([]IntermediateCert, 0, len(byFingerprint))
	for _, c := range byFingerprint {
		result = append(result, c)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Fingerprint < result[j].Fingerprint })

	return result
}

// IntermediateCertsPlugin periodically reports the intermediate certificates returned by its collector.
type IntermediateCertsPlugin struct {
	agent.PluginCommon

	frequency    time.Duration
	includeRoots bool
	collect      IntermediateCertsCollector
}

// NewIntermediateCertsPlugin creates the plugin using the platform specific collector.
func NewIntermediateCertsPlugin(
	ctx agent.AgentContext,
	collect IntermediateCertsCollector,
) agent.Plugin {
	cfg := ctx.Config()

	return &IntermediateCertsPlugin{
		PluginCommon: agent.PluginCommon{ID: IntermediateCertsPluginID, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.IntermediateCertsRefreshSec,
			config.FREQ_MINIMUM_INTERMEDIATE_CERTS,
			config.FREQ_PLUGIN_INTERMEDIATE_CERTS,
			cfg.DisableAllPlugins,
		) * time.Second,
		includeRoots: cfg.IntermediateCertsIncludeRoots,
		collect:      collect,
	}
}

func (p *IntermediateCertsPlugin) dataset() (types.PluginInventoryDataset, error) {
	certs, err := p.collect(p.includeRoots)
	if err != nil {
		err = fmt.Errorf("collecting intermediate certificates: %w", err)
	}

	dataset := make(types.PluginInventoryDataset, 0, len(certs))
	for _, c := range certs {
		dataset = append(dataset, c)
	}

	return dataset, err
}

// Run implements agent.Plugin.
func (p *IntermediateCertsPlugin) Run() {
	if p.frequency <= config.FREQ_DISABLE_SAMPLING {
		iclog.Debug("Disabled.")

		return
	}

	// Introduce some jitter to wait randomly before reporting based on frequency time.
	time.Sleep(config.JitterFrequency(p.frequency))

	refreshTimer := time.NewTicker(p.frequency)
	defer refreshTimer.Stop()

	for {
		dataset, err := p.dataset()
		if err != nil {
			iclog.WithError(err).Warn("Some intermediate certificates could not be collected.")
		}

		// A failed collection without any result must not wipe the previously reported inventory.
		if err == nil || len(dataset) > 0 {
			p.EmitInventory(dataset, entity.NewFromNameWithoutID(p.Context.EntityKey()))
		}

		<-refreshTimer.C
	}
}

// IsCertFileName reports whether a file looks like a certificate file or bundle worth parsing. The numeric
// extensions cover OpenSSL hash links such as "5ad8a5d6.0".
func IsCertFileName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pem", ".crt", ".cer", ".der", ".0", ".1":
		return true
	}

	return false
}

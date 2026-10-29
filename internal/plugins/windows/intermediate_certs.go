// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows

import (
	"crypto/x509"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
)

// The LocalMachine system stores read by the plugin. "CA" is the store shown in certlm.msc as "Intermediate
// Certification Authorities", "Root" as "Trusted Root Certification Authorities" (only read to report roots).
const (
	intermediateCertStore = "CA"
	rootCertStore         = "Root"
)

// NewIntermediateCertsPlugin returns the plugin reporting the intermediate certificates found in the LocalMachine
// "CA" certificate store.
func NewIntermediateCertsPlugin(ctx agent.AgentContext) agent.Plugin {
	return common.NewIntermediateCertsPlugin(ctx, func(includeRoots bool) ([]common.IntermediateCert, error) {
		stores := []string{intermediateCertStore}
		if includeRoots {
			stores = append(stores, rootCertStore)
		}

		return collectIntermediateCerts(stores, includeRoots)
	})
}

func collectIntermediateCerts(stores []string, includeRoots bool) ([]common.IntermediateCert, error) {
	var (
		certs []common.IntermediateCert
		errs  []error
	)

	for _, name := range stores {
		storeCerts, err := readCertStore(name, includeRoots)
		if err != nil {
			errs = append(errs, err)
		}

		certs = append(certs, storeCerts...)
	}

	return common.DedupeIntermediateCerts(certs), errors.Join(errs...)
}

func readCertStore(name string, includeRoots bool) ([]common.IntermediateCert, error) {
	storeName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("invalid store name %q: %w", name, err)
	}

	// Read-only: the plugin must never modify the store.
	store, err := windows.CertOpenStore(
		windows.CERT_STORE_PROV_SYSTEM_W,
		0,
		0,
		windows.CERT_SYSTEM_STORE_LOCAL_MACHINE|windows.CERT_STORE_READONLY_FLAG|windows.CERT_STORE_OPEN_EXISTING_FLAG,
		uintptr(unsafe.Pointer(storeName)),
	)
	if err != nil {
		return nil, fmt.Errorf("opening certificate store %q: %w", name, err)
	}
	defer windows.CertCloseStore(store, 0) //nolint:errcheck

	var (
		certs []common.IntermediateCert
		ctx   *windows.CertContext
	)

	source := "LocalMachine\\" + name

	for {
		// Passing the previous context makes the call free it; the last call (nil result) leaves nothing to free.
		ctx, err = windows.CertEnumCertificatesInStore(store, ctx)
		if err != nil {
			// Enumeration ends with CRYPT_E_NOT_FOUND, which is not a failure.
			if errors.Is(err, windows.Errno(windows.CRYPT_E_NOT_FOUND)) {
				break
			}

			return certs, fmt.Errorf("enumerating certificate store %q: %w", name, err)
		}

		if ctx == nil {
			break
		}

		// The encoded certificate is owned by the context, copy it before it's released.
		der := append([]byte(nil), unsafe.Slice(ctx.EncodedCert, ctx.Length)...)

		cert, parseErr := x509.ParseCertificate(der)
		if parseErr != nil {
			continue
		}

		if c, ok := common.NewCACert(cert, source, includeRoots); ok {
			certs = append(certs, c)
		}
	}

	return certs, nil
}

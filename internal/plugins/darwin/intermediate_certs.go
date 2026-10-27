// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package darwin

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
)

const securityCmdTimeout = 30 * time.Second

// intermediateCertKeychains are the machine-wide keychains read by the plugin. Certificates trusted for all users
// (manually added or pushed by an MDM profile) live in System.keychain. Apple's SystemRootCertificates.keychain only
// holds root certificates, and per-user keychains are out of scope for a host level agent.
//
//nolint:gochecknoglobals
var intermediateCertKeychains = []string{"/Library/Keychains/System.keychain"}

// keychainReader returns the PEM encoded certificates of a keychain.
type keychainReader func(keychain string) ([]byte, error)

// NewIntermediateCertsPlugin returns the plugin reporting the intermediate certificates found in the machine-wide
// macOS keychains.
func NewIntermediateCertsPlugin(ctx agent.AgentContext) agent.Plugin {
	return common.NewIntermediateCertsPlugin(ctx, func(includeRoots bool) ([]common.IntermediateCert, error) {
		return collectIntermediateCerts(intermediateCertKeychains, readKeychain, includeRoots)
	})
}

// readKeychain dumps the certificates of a keychain as PEM using the system `security` tool. Only certificates are
// listed, private keys are never exported.
func readKeychain(keychain string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), securityCmdTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "/usr/bin/security", "find-certificate", "-a", "-p", keychain).Output()
	if err != nil {
		return nil, fmt.Errorf("reading keychain %s: %w", keychain, err)
	}

	return out, nil
}

func collectIntermediateCerts(keychains []string, read keychainReader, includeRoots bool) ([]common.IntermediateCert, error) {
	var (
		certs []common.IntermediateCert
		errs  []error
	)

	for _, keychain := range keychains {
		pemData, err := read(keychain)
		if err != nil {
			errs = append(errs, err)

			continue
		}

		certs = append(certs, common.ParseIntermediateCerts(pemData, keychain, includeRoots)...)
	}

	return common.DedupeIntermediateCerts(certs), errors.Join(errs...)
}

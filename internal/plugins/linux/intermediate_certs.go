// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package linux

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/plugins/common"
)

// maxCertFileSize bounds how much of a single file is read. Full CA bundles are well below 1 MiB.
const maxCertFileSize = 8 << 20

// intermediateCertDirs are the system CA locations of the main distribution families. The extracted bundles are
// included on purpose: intermediates trusted via anchors end up there, deduplicated by fingerprint.
//
//nolint:gochecknoglobals
var intermediateCertDirs = []string{
	"/etc/ssl/certs",                         // Debian, Ubuntu, Alpine, SUSE
	"/usr/local/share/ca-certificates",       // Debian, Ubuntu: locally added
	"/etc/pki/ca-trust/source/anchors",       // RHEL, Fedora, Amazon Linux: locally added
	"/etc/pki/ca-trust/extracted/pem",        // RHEL, Fedora, Amazon Linux: system bundle
	"/etc/pki/tls/certs",                     // RHEL, Fedora, Amazon Linux: legacy bundle location
	"/etc/pki/trust/anchors",                 // SUSE: locally added
	"/usr/share/pki/trust/anchors",           // SUSE: packaged
	"/usr/share/pki/ca-trust-source/anchors", // RHEL family: packaged
}

// NewIntermediateCertsPlugin returns the plugin reporting the intermediate certificates found in the system CA
// directories.
func NewIntermediateCertsPlugin(ctx agent.AgentContext) agent.Plugin {
	return common.NewIntermediateCertsPlugin(ctx, func(includeRoots bool) ([]common.IntermediateCert, error) {
		return collectIntermediateCerts(intermediateCertDirs, includeRoots)
	})
}

func collectIntermediateCerts(dirs []string, includeRoots bool) ([]common.IntermediateCert, error) {
	var (
		certs []common.IntermediateCert
		errs  []error
	)

	for _, dir := range dirs {
		// Symlinked directories (e.g. /etc/ssl/certs on some distros) are resolved so the walk enters them.
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("resolving %s: %w", dir, err))
			}

			continue
		}

		walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				errs = append(errs, err)

				return nil //nolint:nilerr // keep walking, report at the end
			}

			if entry.IsDir() || !common.IsCertFileName(entry.Name()) {
				return nil
			}

			// The hash links of /etc/ssl/certs are symlinks to regular files; Stat follows them.
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() || info.Size() > maxCertFileSize {
				return nil //nolint:nilerr // broken links and special files are skipped
			}

			data, err := os.ReadFile(path)
			if err != nil {
				errs = append(errs, err)

				return nil
			}

			certs = append(certs, common.ParseIntermediateCerts(data, path, includeRoots)...)

			return nil
		})
		if walkErr != nil {
			errs = append(errs, walkErr)
		}
	}

	return common.DedupeIntermediateCerts(certs), errors.Join(errs...)
}

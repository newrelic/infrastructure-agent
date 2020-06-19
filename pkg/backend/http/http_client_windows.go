// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"crypto/x509"
)

// since Go 1.8, Go can't properly load the System root certificates on windows.
// For more info, search Golang issues 16736 and 18609
func systemCertPool() *x509.CertPool {
	plog.Warn("The Windows Infrastructure agent can't load the system root certificates. If you have set up the" +
		" 'ca_bundle_file' or 'ca_bundle_dir' configuration options, you will need to manually download the New Relic" +
		" site certificate and store it into your CA bundle dir")
	return x509.NewCertPool()
}

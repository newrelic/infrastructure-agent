// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import "crypto/x509"

func systemCertPool() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		plog.WithError(err).Warn("Can't load load the system root certificates. If you have set up the" +
			" 'ca_bundle_file' or 'ca_bundle_dir' configuration options, you will need to manually download the New Relic" +
			" site certificate and store it into your CA bundle dir")
		pool = x509.NewCertPool()
	}
	return pool
}

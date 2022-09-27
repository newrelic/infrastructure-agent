// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
)

func addMTLS(server *http.Server, CAPath string) error {
	caCertFile, err := ioutil.ReadFile(CAPath)
	if err != nil {
		return fmt.Errorf("loading CA from %q: %w", CAPath, err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCertFile)

	// serve on port 9090 of local host
	server.TLSConfig = &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  certPool,
		MinVersion: tls.VersionTLS12,
	}

	return nil
}

package httpapi

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
)

func tlsServer(addr string, CAPath string, handler http.Handler) (*http.Server, error) {
	caCertFile, err := ioutil.ReadFile(CAPath)
	if err != nil {
		return nil, fmt.Errorf("loading CA from %q: %w", CAPath, err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCertFile)

	// serve on port 9090 of local host
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		TLSConfig: &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  certPool,
			MinVersion: tls.VersionTLS12,
		},
	}

	return server, nil
}

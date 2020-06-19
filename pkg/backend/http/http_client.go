// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

func GetHttpClient(
	httpTimeout time.Duration,
	transport *http.Transport,
) *http.Client {
	return &http.Client{
		Timeout:   httpTimeout,
		Transport: transport,
	}
}

func getCertPool(certFile string, certDirectory string) *x509.CertPool {
	hlog := plog.WithFields(logrus.Fields{
		"action":    "getCertPool",
		"file":      certFile,
		"directory": certDirectory,
	})

	caCertPool := systemCertPool()

	if certFile != "" {
		caCert, err := ioutil.ReadFile(certFile)
		if err != nil {
			hlog.WithError(err).Error("can't read certificate file")
			os.Exit(1)
		}

		ok := caCertPool.AppendCertsFromPEM(caCert)
		if !ok {
			hlog.Error("certificate could not be appended")
		}
	}
	if certDirectory != "" {
		files, err := ioutil.ReadDir(certDirectory)
		if err != nil {
			log.WithError(err).Error("can't read certificate directory")
			os.Exit(1)
		}

		for _, f := range files {
			if strings.Contains(f.Name(), ".pem") {
				caCertFilePath := filepath.Join(certDirectory + "/" + f.Name())
				caCert, err := ioutil.ReadFile(caCertFilePath)
				if err != nil {
					log.WithField("file", f.Name()).WithError(err).Error("can't read certificate file")
					os.Exit(1)
				}
				ok := caCertPool.AppendCertsFromPEM(caCert)
				if !ok {
					hlog.WithField("file", f.Name()).Error("certificate could not be appended")
				}
			}
		}
	}
	return caCertPool
}

// Client sends a request and returns a response or error.
type Client func(req *http.Request) (*http.Response, error)

// NullHttpClient client discarding all the requests and returning empty objects.
var NullHttpClient = func(req *http.Request) (res *http.Response, err error) {
	return
}

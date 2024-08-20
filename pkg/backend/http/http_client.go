// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//nolint:wrapcheck
package http

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

var ErrUnexepectedResponseCode = errors.New("endpoint returned and unexpected response code")

func GetHttpClient(
	httpTimeout time.Duration,
	transport http.RoundTripper,
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
				caCertFilePath := filepath.Join(certDirectory, f.Name())
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
	_, _ = ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	return
}

func CheckEndpointReachability(
	ctx context.Context,
	logger log.Entry,
	endpointURL string,
	license string,
	userAgent string,
	agentID string,
	timeout time.Duration,
	transport http.RoundTripper,
) (bool, error) {
	var timedOut bool

	request, err := buildRequest(ctx, endpointURL, "HEAD", userAgent, license, agentID)
	if err != nil {
		return false, err
	}

	client := GetHttpClient(timeout, transport)

	// all status codes are acceptable as request has been replied by the endpoint
	resp, err := client.Do(request)
	if err != nil {
		if e2, ok := err.(net.Error); ok && (e2.Timeout() || e2.Temporary()) {
			timedOut = true
		}
		if _, ok := err.(*url.Error); ok {
			logger.WithError(err).
				WithField("userAgent", userAgent).
				WithField("timeout", timeout).
				WithField("url", endpointURL).
				Debug("URL Error detected, may be configuration problem or network connectivity issue.")
			timedOut = true
		}
	}

	if resp != nil {
		_ = resp.Body.Close()
	}

	return timedOut, err
}

func CheckEndpointHealthiness(
	ctx context.Context,
	endpointURL string,
	license string,
	userAgent string,
	agentID string,
	timeout time.Duration,
	transport http.RoundTripper,
) (bool, error) {
	request, err := buildRequest(ctx, endpointURL, "GET", userAgent, license, agentID)
	if err != nil {
		return false, err
	}

	client := GetHttpClient(timeout, transport)

	resp, err := client.Do(request)
	if err != nil {
		return false, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return false, fmt.Errorf("%w, status_code: %d", ErrUnexepectedResponseCode, resp.StatusCode)
	}

	return true, nil
}

func buildRequest(ctx context.Context, endpointURL, method, userAgent, license, agentID string) (*http.Request, error) {
	request, err := http.NewRequest(method, endpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to prepare availability request: %v, error: %w", request, err)
	}

	request = request.WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set(LicenseHeader, license)
	request.Header.Set(EntityKeyHeader, agentID)

	return request, nil
}

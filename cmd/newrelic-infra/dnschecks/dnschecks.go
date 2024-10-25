// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dnschecks

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	http2 "github.com/newrelic/infrastructure-agent/pkg/http"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

const (
	logPrefix     = " ====== "
	dialerTimeout = 10000 // 10000 milliseconds = 10 seconds
)

func RunChecks(
	url string,
	timeout string,
	transport http.RoundTripper,
	logger log.Entry,
) error {
	networkChecks := []func(string, time.Duration, http.RoundTripper, log.Entry) (bool, error){
		checkEndpointReachable,
		checkEndpointReachableDefaultTransport,
		checkEndpointReachableDefaultHTTPHeadClient,
		checkEndpointReachableCustomDNS,
		checkEndpointReachableGoResolverCustom,
	}

	startupConnectionTimeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		// This should never happen, as the correct format is checked
		// during NormalizeConfig.
		logger.WithError(err).Error("Wrong startup_connection_timeout format")
		return err
	}

	for _, networkCheck := range networkChecks {
		_, testErr := networkCheck(url, startupConnectionTimeoutDuration, transport, logger)

		if testErr != nil {
			logger.Error(testErr.Error())
		}
	}
	return nil
}

func startLogMessage(logger log.Entry, testName string) {
	logger.Info(logPrefix + strings.ToUpper("Checking endpoint reachability using "+testName) + logPrefix)
}

func endLogMessage(logger log.Entry, testName string, err error) {
	if err != nil {
		logger.WithError(err).Error(logPrefix + strings.ToUpper("Endpoint reachability using "+testName+" FAILED") + logPrefix)
	} else {
		logger.Info(logPrefix + strings.ToUpper("Endpoint reachability using "+testName+" SUCCEED") + logPrefix)
	}
}

func checkEndpointReachable(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	startLogMessage(logger, "configured agent's HTTP client")
	var request *http.Request
	if request, err = http.NewRequest("HEAD", collectorURL, nil); err != nil {
		return false, fmt.Errorf("unable to prepare reachability request: %v, error: %s", request, err)
	}
	request = http2.WithTracer(request, "checkEndpointReachable")
	client := backendhttp.GetHttpClient(timeout, transport)
	var resp *http.Response

	if resp, err = client.Do(request); err != nil {
		if e2, ok := err.(net.Error); ok && (e2.Timeout() || e2.Temporary()) {
			timedOut = true
		}
		if errURL, ok := err.(*url.Error); ok {
			err = fmt.Errorf("URL error detected. May be a configuration problem or a network connectivity issue.: %w", errURL)
			timedOut = true
		}
	}

	if resp != nil {
		resp.Body.Close()
	}

	endLogMessage(logger, "configured agent's HTTP client", err)

	return
}

func checkEndpointReachableDefaultTransport(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	startLogMessage(logger, "plain HTTP transport")
	var req *http.Request
	client := backendhttp.GetHttpClient(timeout, http.DefaultTransport)
	req, err = http.NewRequest("HEAD", collectorURL, nil)
	if err != nil {
		logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
	} else {
		req = http2.WithTracer(req, "checkEndpointReachable")
		var resp *http.Response
		resp, err = client.Do(req)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("Request for %s failed", collectorURL))
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	endLogMessage(logger, "plain HTTP transport", err)

	return
}

func checkEndpointReachableDefaultHTTPHeadClient(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	startLogMessage(logger, "plain HEAD request")
	_, err = http.Head(collectorURL) //nolint

	endLogMessage(logger, "plain HEAD request", err)
	return
}

func checkEndpointReachableGoResolverCustom(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	startLogMessage(logger, "Golang DNS custom resolver")
	var req *http.Request
	req, err = http.NewRequest("HEAD", collectorURL, nil)
	if err != nil {
		logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
	} else {
		customResolver := &net.Resolver{
			PreferGo:     true,
			Dial:         nil,
			StrictErrors: false,
		}
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  customResolver,
		}
		customTransport := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		client := http.Client{
			Transport:     customTransport,
			Jar:           nil,
			Timeout:       timeout,
			CheckRedirect: nil,
		}
		req = http2.WithTracer(req, "checkEndpointReachableGoResolverCustom")
		var resp *http.Response
		resp, err = client.Do(req)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("Request for %s failed", collectorURL))
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	endLogMessage(logger, "Golang DNS custom resolver", err)
	return
}

func checkEndpointReachableCustomDNS(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	startLogMessage(logger, "public DNS server")
	var req *http.Request
	req, err = http.NewRequest("HEAD", collectorURL, nil)
	if err != nil {
		logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
	} else {
		customResolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				//nolint:exhaustruct
				d := net.Dialer{
					Timeout: time.Millisecond * time.Duration(dialerTimeout),
				}

				//nolint:wrapcheck
				return d.DialContext(ctx, network, "1.1.1.1:53")
			},
			StrictErrors: false,
		}
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  customResolver,
		}
		customTransport := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          1,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		client := http.Client{
			Transport:     customTransport,
			Jar:           nil,
			CheckRedirect: nil,
			Timeout:       timeout,
		}
		req = http2.WithTracer(req, "checkEndpointReachableCustomDNS")
		var resp *http.Response
		resp, err = client.Do(req)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("Request for %s failed", collectorURL))
		}

		if resp != nil {
			resp.Body.Close()
		}
	}

	endLogMessage(logger, "public DNS server", err)
	return
}

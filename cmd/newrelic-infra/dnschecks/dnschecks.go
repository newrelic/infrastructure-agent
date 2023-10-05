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
	LOG_PREFIX = " ====== "
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
	logger.Info(LOG_PREFIX + strings.ToUpper("Checking endpoint reachability using "+testName) + LOG_PREFIX)
}

func endLogMessage(logger log.Entry, testName string, err error) {
	if err != nil {
		logger.WithError(err).Error(LOG_PREFIX + strings.ToUpper("Endpoint reachability using "+testName+" FAILED") + LOG_PREFIX)
	} else {
		logger.Info(LOG_PREFIX + strings.ToUpper("Endpoint reachability using "+testName+" SUCCEED") + LOG_PREFIX)
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
	if _, err = client.Do(request); err != nil {
		if e2, ok := err.(net.Error); ok && (e2.Timeout() || e2.Temporary()) {
			timedOut = true
		}
		if errURL, ok := err.(*url.Error); ok {
			err = fmt.Errorf("URL error detected. May be a configuration problem or a network connectivity issue.: %w", errURL)
			timedOut = true
		}
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
		_, err = client.Do(req)
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
	_, err = http.Head(collectorURL)
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
		resolver := net.DefaultResolver
		resolver.PreferGo = true
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  resolver,
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
		client := http.Client{}
		client.Transport = customTransport
		req = http2.WithTracer(req, "checkEndpointReachable")
		_, err = http.DefaultClient.Do(req)
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
		resolver := net.DefaultResolver
		resolver.PreferGo = true
		resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, "1.1.1.1:53")
		}
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  resolver,
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
		client := backendhttp.GetHttpClient(timeout, customTransport)
		req, err = http.NewRequest("HEAD", collectorURL, nil)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
		} else {
			req = http2.WithTracer(req, "testing")
			_, err = client.Do(req)
		}
	}
	endLogMessage(logger, "public DNS server", err)
	return
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dnschecks

import (
	"context"
	"fmt"
	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	http2 "github.com/newrelic/infrastructure-agent/pkg/http"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"time"
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

func checkEndpointReachable(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	logger.Info("Checking endpoint reachability using default_agent_implementation")
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
			logger.WithError(errURL).Warn("URL error detected. May be a configuration problem or a network connectivity issue.")
			timedOut = true
		}
		logger.WithError(err).Warn("default_agent_implementation: FAIL")
	} else {
		logger.WithError(err).Warn("default_agent_implementation: OK")
	}
	logger.Info("End default_agent_implementation")

	return
}

func checkEndpointReachableDefaultTransport(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	logger.Info("Checking endpoint reachability using default_transport")
	var req *http.Request
	var resp *http.Response
	client := backendhttp.GetHttpClient(timeout, http.DefaultTransport)
	req, err = http.NewRequest("HEAD", collectorURL, nil)
	if err != nil {
		logrus.WithError(err).Error(fmt.Sprintf("cannot Create request for %s", collectorURL))
	} else {
		req = http2.WithTracer(req, "checkEndpointReachable")
		resp, err = client.Do(req)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("cannot Head Default transport With tracer %s", collectorURL))
		} else {
			logrus.WithField("StatusCode", resp.StatusCode).Info("default_transport : OK")
		}
	}
	logger.Info("End default_transport")

	return
}

func checkEndpointReachableDefaultHTTPHeadClient(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	logger.Info("Checking endpoint reachability using default_http_head_client")
	var resp *http.Response
	resp, err = http.Head(collectorURL)
	if err != nil {
		logrus.WithError(err).Error(fmt.Sprintf("default_http_head_client: FAIL"))
	} else {
		logrus.WithField("StatusCode", resp.StatusCode).Info("default_http_head_client: OK")
	}
	logger.Info("End default_http_head_client")
	return
}

func checkEndpointReachableGoResolverCustom(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	logger.Info("Checking endpoint reachability using prefer_go_resolver_custom_transport")
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
		var response *http.Response
		response, err = http.DefaultClient.Do(req)
		if err != nil {
			logrus.WithError(err).Error(fmt.Sprintf("prefer_go_resolver_custom_transport: FAIL"))
		} else {
			logrus.WithField("statusCode", response.StatusCode).Info("prefer_go_resolver_custom_transport: OK")
		}
	}
	logger.Info("End prefer_go_resolver_custom_transport")
	return
}

func checkEndpointReachableCustomDNS(
	collectorURL string,
	timeout time.Duration,
	transport http.RoundTripper,
	logger log.Entry,
) (timedOut bool, err error) {

	logger.Info("Checking endpoint reachability using custom_dns_resolver")
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
			return d.DialContext(ctx, network, "8.8.8.8:53")
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
			resp, err := client.Do(req)
			if err != nil {
				logrus.WithError(err).Error(fmt.Sprintf("cannot Head Default transport With tracer %s", collectorURL))
			} else {
				logrus.WithField("StatusCode", resp.StatusCode).Info("custom_dns_resolver : OK")
			}
		}
	}
	logger.Info("End custom_dns_resolver")
	return
}

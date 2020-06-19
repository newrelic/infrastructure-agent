// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/sirupsen/logrus"
)

const (
	sourceHttpsProxy = "HTTPS_PROXY environment variable"
	sourceProxy      = "proxy configuration option"
	sourceHttpProxy  = "HTTP_PROXY environment variable"
)

// function type that can be assigned to transport.Proxy
type proxyFunc func(*http.Request) (*url.URL, error)

func proxy(u *url.URL) proxyFunc {
	return func(*http.Request) (*url.URL, error) {
		return u, nil
	}
}

func proxyWithError(err error) proxyFunc {
	return func(*http.Request) (*url.URL, error) {
		return nil, err
	}
}

func defaultHttpTransport(
	certFile string,
	certDirectory string,
	httpTimeout time.Duration,
	p proxyFunc,
) *http.Transport {
	var cfg *tls.Config
	if certFile != "" || certDirectory != "" {
		cfg = &tls.Config{RootCAs: getCertPool(certFile, certDirectory)}
	}
	// go default Http Transport
	return &http.Transport{
		Proxy:                 p,
		DialContext:           (&net.Dialer{Timeout: httpTimeout, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   httpTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       cfg,
	}
}

// Proxy configuration, storing the URL of the proxy (nil if there is no proxy), or an error in case the URL is wrongly
// formed. It also returns the dialer to be used for legacy HTTPS connections
type proxyConfig struct {
	source      string
	raw         string
	forceSchema string
}

func (p proxyConfig) isEmpty() bool {
	return p.raw == ""
}

var plog = log.WithComponent("ProxyDialer")

// BuildProxy gets the proxy configuration from the configuration and the environment, according to the following
// priorities (from larger to lower priority):
//
// 1. NRIA_PROXY env var / proxy config option
// 2. HTTPS_PROXY env var
// 3. HTTP_PROXY env var
//
// If the configuration option ignore_system_proxy is set, it ignores the HTTPS_PROXY and HTTP_PROXY configuration
// If the configuration option proxy_validate_certificates is set, it will force the HTTPS proxy options to verify the
// certificates
func proxyByPriority(cfg *config.Config) proxyConfig {
	// this includes NRIA_PROXY and proxy
	if cfg.Proxy != "" {
		return proxyConfig{
			source: sourceProxy,
			raw:    cfg.Proxy,
		}
	}
	if httpsRaw := os.Getenv("HTTPS_PROXY"); httpsRaw != "" && !cfg.IgnoreSystemProxy {
		forceSchema := ""
		if cfg.ProxyValidateCerts {
			// For backwards-compatibility, we don't force HTTPS_PROXY to be actually https unless we use the
			// more recent proxy_validate_certificates option
			forceSchema = "https"
		}
		return proxyConfig{
			source:      sourceHttpsProxy,
			raw:         httpsRaw,
			forceSchema: forceSchema,
		}
	}
	if raw := os.Getenv("HTTP_PROXY"); raw != "" && !cfg.IgnoreSystemProxy {
		return proxyConfig{
			source: sourceHttpProxy,
			raw:    raw,
		}
	}

	return proxyConfig{}
}

// BuildTransport creates an http.Transport. If there is a configured proxy, in the configuration and environment,
// it configures the transport to use the proxy, according to the following priorities (from larger to lower priority):
//
// 1. HTTPS_PROXY env var
// 2. NRIA_PROXY env var / proxy config option
// 3. HTTP_PROXY env var
//
// If the configuration option ignore_system_proxy is set, it ignores the HTTPS_PROXY and HTTP_PROXY configuration
// If the configuration option proxy_validate_certificates is set, it will force the HTTPS proxy options to verify the
// certificates
func BuildTransport(cfg *config.Config, timeout time.Duration) *http.Transport {
	proxyConfig := proxyByPriority(cfg)

	if proxyConfig.isEmpty() {
		return defaultHttpTransport(
			cfg.CABundleFile,
			cfg.CABundleDir,
			timeout,
			nil, // no proxy configuration
		)
	}

	u, err := url.Parse(proxyConfig.raw)
	if err != nil || !hasValidScheme(u.Scheme) {
		// (taken from "ProxyFromEnvironment" Go standard library function)
		// proxy was bogus. Try prepending "http://" to it and
		// see if that parses correctly. If not, we fall
		// through and complain about the original one.
		u, err = url.Parse("http://" + proxyConfig.raw)
	}
	if err != nil {
		err = fmt.Errorf("invalid proxy address %q: %v", proxyConfig.raw, err)
		logrus.WithError(err).Error()
		return defaultHttpTransport(
			cfg.CABundleFile,
			cfg.CABundleDir,
			timeout,
			proxyWithError(err))
	}

	if proxyConfig.forceSchema != "" && proxyConfig.forceSchema != u.Scheme {
		err = fmt.Errorf("schema from %s must be %q", proxyConfig.source, proxyConfig.forceSchema)
		logrus.WithError(err).Error()
		return defaultHttpTransport(
			cfg.CABundleFile,
			cfg.CABundleDir,
			timeout,
			proxyWithError(err))
	}

	t := defaultHttpTransport(
		cfg.CABundleFile,
		cfg.CABundleDir,
		timeout,
		proxy(u),
	)

	if cfg.ProxyValidateCerts {
		if u.Scheme == "https" {
			t.DialTLS = fullTLSToHTTPConnectFallbackDialer(t)
		} else {
			plog.WithField("scheme", u.Scheme).
				Warn("proxy_validate_certificates option is set to 'true', but the Proxy URL scheme is not HTTPS")
		}
	} else if u.Scheme == "https" {
		plog.WithField("scheme", u.Scheme).
			Info("If using an HTTPS proxy, for enhanced security it is recommended to configure the agent with the" +
				" 'proxy_validate_certificates' configuration option set to true")
		t.DialTLS = fallbackDialer(t)
	}
	return t
}

func hasValidScheme(s string) bool {
	return s == "http" || s == "https" || s == "socks5"
}

// Dial verifier implements the transport.Dialer interface to provide backwards compatibility with Go 1.9 proxy
// implementation.
//
// It does the following process:
//
// 1. Tries to normally connect to an HTTPS proxy
// 2. If succeeds, uses the normal `tls.Dial` function in further connections
// 3. If an Unknown Authority Error is returned, InsecureSkipVerify is set to true and we continue using
//    `tls.Dial` for the following connections.
// 4. If the secure connection is not accepted, we use an unsecured "Go1.9-like" dialer that does not
//    performs the TLS handshake.
//
// IMPORTANT: This verification mode should be only done with legacy proxy implementation, where the
// proxy_validate_certificates configuration option is set to false, to avoid breaking changes with legacy users since the
// update from Go 1.9 to Go 1.10.
func fallbackDialer(transport *http.Transport) func(network string, addr string) (net.Conn, error) {
	return func(network string, addr string) (conn net.Conn, e error) {
		// test the tlsDialer with normal configuration
		plog.Debug("Dialing with usual, secured configuration.")
		dialer := tlsDialer(transport)
		conn, err := dialer(network, addr)
		if err == nil {
			plog.Debug("Usual, secured configuration worked as expected. Defaulting to it.")
			// if worked, we will use tlsDialer directly from now on
			transport.DialTLS = dialer
			return conn, err
		}
		switch err.(type) {
		case x509.UnknownAuthorityError:
			plog.WithError(err).Debug("Usual, secured configuration did not work as expected. Retrying with verification skip.")
			// if in the previous request we received an authority error, we skip verification and
			// continue using tlsDialer directly from now on
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}
			transport.TLSClientConfig.InsecureSkipVerify = true

			// we will use tlsDialer directly from now on, with the insecure skip configuration
			transport.DialTLS = tlsDialer(transport)
			return transport.DialTLS(network, addr)
		case tls.RecordHeaderError:
			plog.WithError(err).Debug("Usual, secured configuration did not work as expected. Retrying with HTTP dialing.")
			// if the problem was due to a non-https connection, we use a non-tls dialer directly
			// from now on
			transport.DialTLS = nonTLSDialer
			return transport.DialTLS(network, addr)
		default:
			return conn, err
		}
	}
}

// It tries to initiate TLS handshake with proxy out of any HTTP context, some proxies allow this,
// some others don't, also HTTP/S firewalls may block this plain TCP TLS handshake.
// In case this fails, proxy/firewall is configured to only handle HTTP/S traffic,
// therefore an HTTP CONNECT initiator request is required prior to trigger the proxy TLS handshake.
func fullTLSToHTTPConnectFallbackDialer(t *http.Transport) func(network string, addr string) (net.Conn, error) {
	return func(network string, addr string) (conn net.Conn, e error) {
		plog.Debug("Dialing to proxy via TLS")
		dialer := tlsDialer(t)
		conn, err := dialer(network, addr)
		if err == nil {
			plog.Debug("Usual, secured configuration worked as expected. Defaulting to it.")
			t.DialTLS = dialer
			return conn, nil
		}

		switch err.(type) {
		case tls.RecordHeaderError:
			plog.WithError(err).Debug("TLS handshake cannot be established. Retrying with HTTP CONNECT")
			t.DialTLS = nonTLSDialer
			return t.DialTLS(network, addr)
		default:
			return conn, err
		}
	}
}

// tlsDialer wraps the standard library tls.Dial function
func tlsDialer(transport *http.Transport) func(network string, addr string) (net.Conn, error) {
	return func(network string, addr string) (conn net.Conn, e error) {
		return tls.Dial(network, addr, transport.TLSClientConfig)
	}
}

// nonTlsDial mimics the tls.DialWithDialer function, but without performing TLS handshakes
func nonTLSDialer(network, addr string) (net.Conn, error) {
	dialer := new(net.Dialer)
	// We want the Timeout and Deadline values from dialer to cover the
	// whole process: TCP connection and TLS handshake. This means that we
	// also need to start our own timers now.
	timeout := dialer.Timeout

	if !dialer.Deadline.IsZero() {
		deadlineTimeout := time.Until(dialer.Deadline)
		if timeout == 0 || deadlineTimeout < timeout {
			timeout = deadlineTimeout
		}
	}

	var errChannel chan error

	if timeout != 0 {
		errChannel = make(chan error, 2)
		time.AfterFunc(timeout, func() {
			errChannel <- timeoutError{}
		})
	}

	return dialer.Dial(network, addr)
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "tls: DialWithDialer timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

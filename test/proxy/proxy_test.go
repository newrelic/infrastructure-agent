// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build proxytests

package proxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/newrelic/infrastructure-agent/test/proxy/minagent"
	"github.com/newrelic/infrastructure-agent/test/proxy/testsetup"
)

func TestNriaHttpsProxyVerifiedConnection(t *testing.T) {
	// given an agent with proxy environment configuration options
	require.NoError(t, restartAgent(minagent.ConfigOptions{Environment: map[string]string{
		"HTTPS_PROXY":                      testsetup.ActualHttpsProxy,
		"NRIA_PROXY_VALIDATE_CERTIFICATES": "true",
		"NRIA_DISPLAY_NAME":                "nria-https-proxy-client",
		"NRIA_CA_BUNDLE_DIR":               "/fullcabundle",
	}}))
	// that sends data to a collector
	require.NoError(t, cleanupCollector())

	// the collector data is bypassed by the https-proxy
	eventCh, errCh := getCollectorEvent()
	select {
	case event := <-eventCh:
		assert.NotEmpty(t, event.Samples)
		assert.Equal(t, "nria-https-proxy-client", findEntityKey(event))
		assert.Equal(t, testsetup.ActualHttpsProxyName, getUsedProxy())
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(timeout):
		assert.Fail(t, "timeout while waiting for a collector event")
	}
}

func TestNriaHttpsProxyUnverifiedConnection_MissingCABundle(t *testing.T) {
	// given an agent with proxy environment configuration options
	require.NoError(t, restartAgent(minagent.ConfigOptions{Environment: map[string]string{
		"HTTPS_PROXY":       testsetup.ActualHttpsProxy,
		"NRIA_DISPLAY_NAME": "nria-https-proxy-client-dont-validate",
	}}))
	// that sends data to a collector
	require.NoError(t, cleanupCollector())

	// the collector data is bypassed by the https-proxy
	eventCh, errCh := getCollectorEvent()
	select {
	case event := <-eventCh:
		assert.NotEmpty(t, event.Samples)
		assert.Equal(t, "nria-https-proxy-client-dont-validate", findEntityKey(event))
		assert.Equal(t, testsetup.ActualHttpsProxyName, getUsedProxy())
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(timeout):
		assert.Fail(t, "timeout while waiting for a collector event")
	}
}

// VERY SLOW TEST! (5 seconds)
func TestNriaHttpsProxyVerifiedConnection_MissingCABundle(t *testing.T) {
	// given an agent with proxy environment configuration options
	require.NoError(t, restartAgent(minagent.ConfigOptions{Environment: map[string]string{
		"HTTPS_PROXY":                      testsetup.ActualHttpsProxy,
		"NRIA_PROXY_VALIDATE_CERTIFICATES": "true",
		"NRIA_DISPLAY_NAME":                "nria-https-proxy-client-no-cabundle",
		"NRIA_VERBOSE":                     "1",
	}}))
	// that sends data to a collector
	require.NoError(t, cleanupCollector())

	// the collector data is not submitted because the certificates are not ready
	eventCh, errCh := getCollectorEvent()
	select {
	case event := <-eventCh:
		assert.Fail(t, "data shouldn't have been sent!", "received from proxy %q: %#v", getUsedProxy(), event)
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
	}
}

func TestNriaHttpsProxyConnection_ConfigFile(t *testing.T) {
	// given an agent with proxy environment configuration options and a config file
	require.NoError(t, restartAgent(minagent.ConfigOptions{
		Environment: map[string]string{
			"NRIA_DISPLAY_NAME": "nria-https-proxy-client-file",
		},
		ConfigFile: "/fake-config-httpsproxy-verify.yml",
	}))
	// that sends data to a collector
	require.NoError(t, cleanupCollector())

	// the collector data is bypassed by the https-proxy
	eventCh, errCh := getCollectorEvent()
	select {
	case event := <-eventCh:
		assert.NotEmpty(t, event.Samples)
		assert.Equal(t, "nria-https-proxy-client-file", findEntityKey(event))
		assert.Equal(t, testsetup.ActualHttpsProxyName, getUsedProxy())
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(timeout):
		assert.Fail(t, "timeout while waiting for a collector event")
	}
}

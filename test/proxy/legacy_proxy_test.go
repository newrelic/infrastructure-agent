// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build proxytests

package proxy

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/test/proxy/fakecollector"
	"github.com/newrelic/infrastructure-agent/test/proxy/minagent"
	"github.com/newrelic/infrastructure-agent/test/proxy/testsetup"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const timeout = 10 * time.Second

func TestNoProxyConnection(t *testing.T) {
	// given an agent with default configuration options
	require.NoError(t, restartAgent(minagent.ConfigOptions{Environment: map[string]string{
		"NRIA_DISPLAY_NAME": "default-agent",
	}}))
	// that sends data to a collector
	require.NoError(t, cleanupCollector())

	// the collector receives the data directly, without passing by any proxy
	eventCh, errCh := getCollectorEvent()
	select {
	case event := <-eventCh:
		assert.NotEmpty(t, event.Samples)
		assert.Equal(t, "default-agent", findEntityKey(event))
		assert.Empty(t, getUsedProxy())
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(timeout):
		assert.Fail(t, "timeout while waiting for a collector event")
	}
}

func TestLegacyEnvProxyConnection(t *testing.T) {
	cases := []struct {
		env string
		val string
		exp string // expected proxy
	}{
		{env: "HTTP_PROXY", val: testsetup.HttpProxy, exp: testsetup.HttpProxyName},
		{env: "NRIA_PROXY", val: testsetup.HttpProxy, exp: testsetup.HttpProxyName},
		//// Even with legacy HTTPS_PROXY config, we expect the http proxy to be used
		{env: "HTTPS_PROXY", val: testsetup.HttpsProxy, exp: testsetup.HttpProxyName},
		// With HTTPS urls in an actual HTTPS proxy, the test will pass with legacy configurations (ignoring certs)
		{env: "HTTP_PROXY", val: testsetup.ActualHttpsProxy, exp: testsetup.ActualHttpsProxyName},
		{env: "NRIA_PROXY", val: testsetup.ActualHttpsProxy, exp: testsetup.ActualHttpsProxyName},
		{env: "HTTPS_PROXY", val: testsetup.ActualHttpsProxy, exp: testsetup.ActualHttpsProxyName},
	}

	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			displayName := fmt.Sprintf("proxy-env-agent-%s", c.env)

			// given an agent with proxy environment configuration options
			require.NoError(t, restartAgent(minagent.ConfigOptions{Environment: map[string]string{
				c.env:               c.val,
				"NRIA_DISPLAY_NAME": displayName,
			}}))
			// that sends data to a collector
			require.NoError(t, cleanupCollector())

			// the collector data is bypassed by the http-proxy, always (checks the cached DialTLS works properly)
			for i := 0; i < 3; i++ {
				eventCh, errCh := getCollectorEvent()
				select {
				case event := <-eventCh:
					assert.NotEmpty(t, event.Samples)
					assert.Equal(t, displayName, findEntityKey(event))
					assert.Equal(t, c.exp, getUsedProxy())
				case err := <-errCh:
					assert.NoError(t, err)
				case <-time.After(timeout):
					assert.Fail(t, "timeout while waiting for a collector event")
				}
			}
		})
	}
}

// similar to the previous test, but HTTP_PROXY and HTTPS_PROXY must be ignored.
func TestLegacyEnvProxyConnection_IgnoreSystemProxy(t *testing.T) {
	cases := []struct {
		env           string
		val           string
		expectedProxy string
	}{
		{env: "HTTP_PROXY", val: testsetup.HttpProxy, expectedProxy: ""},
		{env: "NRIA_PROXY", val: testsetup.HttpProxy, expectedProxy: testsetup.HttpProxyName},
		{env: "HTTPS_PROXY", val: testsetup.HttpsProxy, expectedProxy: ""},
	}

	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			displayName := fmt.Sprintf("proxy-env-agent-%s", c.env)

			// given an agent with proxy environment configuration options and ignore_system_proxy = true
			require.NoError(t, restartAgent(minagent.ConfigOptions{Environment: map[string]string{
				c.env:                      c.val,
				"NRIA_DISPLAY_NAME":        displayName,
				"NRIA_IGNORE_SYSTEM_PROXY": "true",
			}}))
			// that sends data to a collector
			require.NoError(t, cleanupCollector())

			// the collector data is bypassed by the http-proxy only for NRIA_PROXY
			eventCh, errCh := getCollectorEvent()
			select {
			case event := <-eventCh:
				assert.NotEmpty(t, event.Samples)
				assert.Equal(t, displayName, findEntityKey(event))
				assert.Equal(t, c.expectedProxy, getUsedProxy())
			case err := <-errCh:
				assert.NoError(t, err)
			case <-time.After(timeout):
				assert.Fail(t, "timeout while waiting for a collector event")
			}
		})
	}
}

func TestLegacyProxyConfConnection(t *testing.T) {
	require.NoError(t, restartAgent(minagent.ConfigOptions{
		ConfigFile: "/fake-config-httpproxy.yml",
		Environment: map[string]string{
			"NRIA_DISPLAY_NAME": "http-confffile-proxy",
		}}))
	// that sends data to a collector
	require.NoError(t, cleanupCollector())

	// the collector data is bypassed by the http-proxy
	eventCh, errCh := getCollectorEvent()
	select {
	case event := <-eventCh:
		assert.NotEmpty(t, event.Samples)
		assert.Equal(t, "http-confffile-proxy", findEntityKey(event))
		assert.Equal(t, testsetup.HttpProxyName, getUsedProxy())
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(timeout):
		assert.Fail(t, "timeout while waiting for a collector event")
	}
}

// This test checks the priorities, by setting different proxy values (the low-priority proxy have a wrong
// URL that would avoid the samples reaching the collector), and verifying that the samples arrived to the collector
// because the correct proxy URL is being actually used.
// Expected order of priorities (from highest to lower priority):
// 1. NRIA_PROXY env var
// 2. proxy option in configuration file
// 3. HTTPS_PROXY env var
// 4. HTTP_PROXY env var
func TestProxyPriorities(t *testing.T) {
	cases := []struct {
		description            string
		highPriorityProxy      string
		highPriorityProxyValue string
		lowPriorityProxy       string
		lowPriorityProxyValue  string
		configFile             string
	}{
		{"HTTPS_PROXY has priority over HTTP_PROXY",
			"HTTPS_PROXY", testsetup.HttpsProxy,
			"HTTP_PROXY", "http://wrong-url:1234",
			""},
		{"proxy has priority over HTTPS_PROXY",
			"", "",
			"HTTPS_PROXY", "http://wrong-url:1234",
			"/fake-config-httpproxy.yml"},
		{"NRIA PROXY has priority over proxy",
			"NRIA_PROXY", testsetup.HttpProxy,
			"", "",
			"/fake-config-wronghttpproxy.yml"},
	}
	for n, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			displayName := fmt.Sprintf("proxy-priority-%d", n)
			require.NoError(t, restartAgent(minagent.ConfigOptions{
				ConfigFile: c.configFile,
				Environment: map[string]string{
					"NRIA_DISPLAY_NAME": displayName,
					c.highPriorityProxy: c.highPriorityProxyValue,
					c.lowPriorityProxy:  c.lowPriorityProxyValue,
				},
			}))
			require.NoError(t, cleanupCollector())

			// the collector data is bypassed by the http-proxy
			eventCh, errCh := getCollectorEvent()
			select {
			case event := <-eventCh:
				assert.NotEmpty(t, event.Samples)
				assert.Equal(t, displayName, findEntityKey(event))
				assert.Equal(t, testsetup.HttpProxyName, getUsedProxy())
			case err := <-errCh:
				assert.NoError(t, err)
			case <-time.After(timeout):
				assert.Fail(t, "timeout while waiting for a collector event")
			}
		})
	}
}

// Proxy Tests helper test functions

// newClient creates a secure https client that skips hostname verification
func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

// restartAgent restarts the testing containerized agent
func restartAgent(config minagent.ConfigOptions) error {
	body, err := json.Marshal(config)
	if err != nil {
		return err
	}
	response, err := http.Post(testsetup.AgentRestart, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected response while restarting agent: %v", response.StatusCode)
	}
	return nil
}

// cleanupCollector cleans the samples queue in the collector, as well as the reported used proxy
func cleanupCollector() error {
	response, err := newClient().Get(testsetup.CollectorCleanup)
	if err != nil {
		return nil
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected response while cleaning up collector: %v", response.StatusCode)
	}
	return nil
}

// getCollectorEvent returns a channel that receives the following event in the containerized fake collector
func getCollectorEvent() (<-chan fakecollector.Request, <-chan error) {
	eventCh := make(chan fakecollector.Request)
	errCh := make(chan error)
	go func() {
		var event fakecollector.Request
		response, err := newClient().Get(testsetup.CollectorNextEvent)
		if err != nil {
			errCh <- err
		}
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusAccepted {
			errCh <- fmt.Errorf("unexpected response while getting collector event: %v", response.StatusCode)
		}
		eventBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			errCh <- err
		}
		err = json.Unmarshal(eventBytes, &event)
		if err != nil {
			errCh <- err
		}
		eventCh <- event
	}()
	return eventCh, errCh
}

// getUsedProxy returns the last proxy that has been used in the collector, or empty if no proxy has been used
func getUsedProxy() string {
	response, err := newClient().Get(testsetup.CollectorUsedProxy)
	if err != nil {
		logrus.WithError(err).Error("getting the last used proxy")
		return ""
	}
	proxy, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logrus.WithError(err).Error("parsing the last used proxy response body")
		return ""
	}
	return string(proxy)
}

// findEntityKey returns the first "entityKey" entry in the array of samples that have been submitted to the
// fake collector
func findEntityKey(from fakecollector.Request) string {
	for _, sample := range from.Samples {
		events, ok := sample["Events"]
		if !ok {
			continue
		}
		cevents, ok := events.([]interface{})
		if !ok {
			continue
		}
		for _, event := range cevents {
			cevent, ok := event.(map[string]interface{})
			if !ok {
				continue
			}
			if entityKey, ok := cevent["entityKey"]; ok {
				return entityKey.(string)
			}
		}
	}
	return ""
}

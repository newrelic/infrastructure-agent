// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build harvest
// +build harvest

package harvest

import (
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/proxy"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyProxyConfigPlugin(t *testing.T) {
	testhelpers.SetupLog()

	// Given an agent with no proxy configuration
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)
	a.RegisterPlugin(proxy.ConfigPlugin(a.Context))
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})
	go a.Run()

	// When the proxy plugin reports the proxy configuration
	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(15 * time.Second):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// Only the proxy_validate_certificates and ignore_system_proxy configuration options are reported
	// with their default values
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/proxy_config",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"proxy_validate_certificates": map[string]interface{}{
					"id":    "proxy_validate_certificates",
					"value": false,
				},
				"ignore_system_proxy": map[string]interface{}{
					"id":    "ignore_system_proxy",
					"value": false,
				},
				"HTTPS_PROXY":    fixture.NilValue,
				"HTTP_PROXY":     fixture.NilValue,
				"proxy":          fixture.NilValue,
				"ca_bundle_dir":  fixture.NilValue,
				"ca_bundle_file": fixture.NilValue,
			},
		},
	})
}

func TestEmptyConfigPlugin(t *testing.T) {
	testhelpers.SetupLog()

	caBundleDir := os.TempDir()
	caBundleFile, err := ioutil.TempFile("", "proxy_test")
	require.NoError(t, err)

	// Given an agent with several proxy configurations
	testClient := ihttp.NewRequestRecorderClient()

	os.Setenv("HTTPS_PROXY", "https://localhost:443")
	defer os.Unsetenv("HTTPS_PROXY")
	os.Setenv("HTTP_PROXY", "http://localhost:443")
	defer os.Unsetenv("HTTP_PROXY")
	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.Proxy = "http://localhost:1234"
		cfg.IgnoreSystemProxy = true
		cfg.ProxyValidateCerts = true
		cfg.CABundleDir = caBundleDir
		cfg.CABundleFile = caBundleFile.Name()
	})
	a.RegisterPlugin(proxy.ConfigPlugin(a.Context))
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})
	go a.Run()

	// When the proxy plugin reports the proxy configuration
	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(10 * time.Second):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// All the options have been reported, without filtering any sensitive data
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/proxy_config",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"proxy_validate_certificates": map[string]interface{}{
					"id":    "proxy_validate_certificates",
					"value": true,
				},
				"ignore_system_proxy": map[string]interface{}{
					"id":    "ignore_system_proxy",
					"value": true,
				},
				"HTTPS_PROXY": map[string]interface{}{
					"id":     "HTTPS_PROXY",
					"scheme": "https",
					"error":  fixture.NilValue,
				},
				"HTTP_PROXY": map[string]interface{}{
					"id":     "HTTP_PROXY",
					"scheme": "http",
					"error":  fixture.NilValue,
				},
				"proxy": map[string]interface{}{
					"id":     "proxy",
					"scheme": "http",
					"error":  fixture.NilValue,
				},
				"ca_bundle_file": map[string]interface{}{
					"id":    "ca_bundle_file",
					"type":  "file",
					"error": fixture.NilValue,
				},
				"ca_bundle_dir": map[string]interface{}{
					"id":    "ca_bundle_dir",
					"type":  "directory",
					"error": fixture.NilValue,
				},
			},
		},
	})
}

func TestWrongConfigPlugin(t *testing.T) {
	const wrongURL = "a%20xxx://localhost"

	testhelpers.SetupLog()

	// Given an agent with wrong proxy configurations
	testClient := ihttp.NewRequestRecorderClient()

	os.Setenv("HTTPS_PROXY", wrongURL)
	defer os.Unsetenv("HTTPS_PROXY")
	os.Setenv("HTTP_PROXY", wrongURL)
	defer os.Unsetenv("HTTP_PROXY")
	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.Proxy = wrongURL
	})
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})
	a.RegisterPlugin(proxy.ConfigPlugin(a.Context))
	go a.Run()

	// When the proxy plugin reports the proxy configuration
	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(5 * time.Second):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// All the options have been reported, without filtering any sensitive data
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/proxy_config",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"HTTPS_PROXY": map[string]interface{}{
					"id":     "HTTPS_PROXY",
					"scheme": fixture.NilValue,
					"error":  fixture.AnyValue,
				},
				"HTTP_PROXY": map[string]interface{}{
					"id":     "HTTP_PROXY",
					"scheme": fixture.NilValue,
					"error":  fixture.AnyValue,
				},
				"proxy": map[string]interface{}{
					"id":     "proxy",
					"scheme": fixture.NilValue,
					"error":  fixture.AnyValue,
				},
			},
		},
	})
}

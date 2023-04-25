// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package core

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	metrics_sender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	fixture_inventory "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/sample"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

// TestHttpHeaders asserts that http requests performed by the agent contains the http headers specified in the configuration.
func TestHttpHeaders_Inventory(t *testing.T) {
	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.Http.Headers["test_key"] = "test_value"
	})

	a.Context.SetAgentIdentity(entity.Identity{
		ID:   10,
		GUID: "abcdef",
	})

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	a.RegisterPlugin(plugins.NewHostAliasesPlugin(a.Context, cloudDetector))
	a.RegisterPlugin(plugins.NewAgentConfigPlugin(*ids.NewPluginID("metadata", "agent_config"), a.Context))

	go a.Run()

	select {
	case req := <-testClient.RequestCh:
		fixture_inventory.AssertRequestContainsInventoryDeltas(t, req, fixture_inventory.ExpectedMetadataDelta)

		assert.EqualValues(t, req.Header, map[string][]string{
			"Content-Type":     {"application/json"},
			"Test_key":         {"test_value"},
			"User-Agent":       {"user-agent"},
			"X-License-Key":    {""},
			"X-Nri-Entity-Key": {"display-name"},
		})
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}
	a.Terminate()
}

func TestHttpHeaders_Samples(t *testing.T) {
	sample := fixture.StorageSample

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.Http.Headers["test_key"] = "test_value"
	})

	sender := metrics_sender.NewSender(a.Context)
	sender.RegisterSampler(fixture.NewSampler(&sample))
	a.RegisterMetricsSender(sender)

	go a.Run()

	req := <-testClient.RequestCh
	a.Terminate()

	fixture.AssertRequestContainsSample(t, req, &sample)

	assert.EqualValues(t, req.Header, map[string][]string{
		"Content-Type":     {"application/json"},
		"Test_key":         {"test_value"},
		"User-Agent":       {"user-agent"},
		"X-License-Key":    {""},
		"X-Nri-Entity-Key": {"display-name"},
	})
}

func TestHttpHeaders_Connect(t *testing.T) {

	cfg := &config.Config{
		DisplayName:              "display-name",
		FirstReapInterval:        time.Millisecond,
		ReapInterval:             time.Millisecond,
		SendInterval:             time.Millisecond,
		FingerprintUpdateFreqSec: 60,
		StartupConnectionRetries: 3,
		StartupConnectionTimeout: "5s",
		OfflineTimeToReset:       config.DefaultOfflineTimeToReset,
		ConnectEnabled:           true,
		Http: config.HttpConfig{
			Headers: map[string]string{
				"Content-Encoding": "gzip2",
			},
		},
	}

	testClient := ihttp.NewRequestRecorderClient()

	transport := ihttp.ToRoundTripper(testClient.Client)
	httpClient := &http.Client{
		Transport: transport,
	}

	a := infra.NewAgentWithConnectClientAndConfig(httpClient, httpClient.Do, cfg)
	go a.Run()

	req := <-testClient.RequestCh
	a.Terminate()

	assert.Equal(t, req.URL.Path, "url/connect")
	assert.EqualValues(t, req.Header, map[string][]string{
		"Content-Encoding": {"gzip", "gzip2"},
		"Content-Type":     {"application/json"},
		"User-Agent":       {"user-agent"},
		"X-License-Key":    {"license"},
	})

}

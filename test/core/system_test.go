// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package core

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"

	metrics_sender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture_inventory "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/sample"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
)

func TestSystemSample(t *testing.T) {
	sample := fixture.SystemSample

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	sender := metrics_sender.NewSender(a.Context)
	sender.RegisterSampler(fixture.NewSampler(&sample))
	a.RegisterMetricsSender(sender)

	go a.Run()

	req := <-testClient.RequestCh
	a.Terminate()

	fixture.AssertRequestContainsSample(t, req, &sample)
}

// Metadata comes from inventory using the agent context as provider
// then this data is processed upstream in the platform to build Insights fields like `agentVersion`.
func TestMetadata(t *testing.T) {
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	a.RegisterPlugin(plugins.NewHostAliasesPlugin(a.Context, cloudDetector))
	a.RegisterPlugin(plugins.NewAgentConfigPlugin(*ids.NewPluginID("metadata", "agent_config"), a.Context))

	go a.Run()

	req := <-testClient.RequestCh
	a.Terminate()

	fixture_inventory.AssertRequestContainsInventoryDeltas(t, req, fixture_inventory.ExpectedMetadataDelta)
}

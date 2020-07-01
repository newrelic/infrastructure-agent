// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package core

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

type dummyPlugin struct {
	agent.PluginCommon
	ticker chan interface{}
	value  string
}

type valueEntry struct {
	Id    string `json:"id"`
	Value string `json:"value"`
}

func (v *valueEntry) SortKey() string {
	return v.Id
}

func newDummyPlugin(initialValue string, context agent.AgentContext) *dummyPlugin {
	return &dummyPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.PluginID{"test", "dummy"},
			Context: context,
		},
		ticker: make(chan interface{}),
		value:  initialValue,
	}
}

func (cp *dummyPlugin) Run() {
	for {
		select {
		case <-cp.ticker:
			dataset := agent.PluginInventoryDataset{
				&valueEntry{Id: "dummy", Value: cp.value},
			}
			cp.EmitInventory(dataset, cp.Context.AgentIdentifier())
		}
	}
}

func (cp *dummyPlugin) Id() ids.PluginID {
	return cp.ID
}

func TestDeltas_BasicWorkflow(t *testing.T) {
	const timeout = 5 * time.Second

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("test/dummy", 1),
		ihttp.AcceptedResponse("test/dummy", 2))
	a := infra.NewAgent(testClient.Client)

	// That runs a plugin
	plugin := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin)

	go a.Run()

	// When the plugin harvests inventory data
	plugin.ticker <- 1

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// The full delta is submitted
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "test/dummy",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"dummy": map[string]interface{}{
					"value": "hello",
				},
			},
		},
	})

	// And if the plugin harvests again the same inventory data
	plugin.ticker <- 1

	// No deltas are sent
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
		buf := new(bytes.Buffer)
		buf.ReadFrom(req.Body)
		bodyStr := buf.String()

		assert.FailNow(t, "no request expected at this point", "req = %v", bodyStr)
	case <-time.After(50 * time.Millisecond):
	}

	// And if the plugin harvests new inventory data
	plugin.value = "goodbye"
	plugin.ticker <- 1

	// A new delta is submitted
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// The partial delta is submitted
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "test/dummy",
			ID:       2,     // The id has been incremented
			FullDiff: false, // FullDiff is now false
			Diff: map[string]interface{}{
				"dummy": map[string]interface{}{
					"value": "goodbye",
				},
			},
		},
	})
}

func TestDeltas_ResendIfFailure(t *testing.T) {
	const timeout = 5 * time.Second

	// Given an agent that fails submitting the deltas in the second invocation
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("test/dummy", 1),
		ihttp.ErrorResponse,
		ihttp.AcceptedResponse("test/dummy", 2))

	a := infra.NewAgent(testClient.Client)

	// That runs a plugin
	plugin := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin)

	go a.Run()

	// When the plugin harvests inventory data
	plugin.ticker <- 1

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// The full delta is submitted
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "test/dummy",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"dummy": map[string]interface{}{
					"value": "hello",
				},
			},
		},
	})

	// And if the plugin harvests new inventory data
	plugin.value = "goodbye"
	plugin.ticker <- 1

	// A new delta is submitted
	select {
	case req = <-testClient.RequestCh:
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// The partial delta is submitted
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "test/dummy",
			ID:       2,     // The id has been incremented
			FullDiff: false, // FullDiff is now false
			Diff: map[string]interface{}{
				"dummy": map[string]interface{}{
					"value": "goodbye",
				},
			},
		},
	})

	// And if the submission failed
	// A new delta is submitted
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// The partial delta is submitted again
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "test/dummy",
			ID:       2,     // The id has been incremented
			FullDiff: false, // FullDiff is now false
			Diff: map[string]interface{}{
				"dummy": map[string]interface{}{
					"value": "goodbye",
				},
			},
		},
	})

}

func TestDeltas_ResendAfterReset(t *testing.T) {
	const timeout = 5 * time.Second

	agentDir, err := ioutil.TempDir("", "prefix")
	if err != nil {
		panic(err)
	}

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.SendInterval = time.Hour
		config.AgentDir = agentDir
	})

	// That runs a plugin
	plugin1 := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin1)

	go a.Run()

	// When the plugin harvests inventory data
	plugin1.ticker <- 1

	// And the agent restarts before data is submitted
	select {
	case <-testClient.RequestCh:
		a.Terminate()
		assert.FailNow(t, "Agent must not send data yet")
	case <-time.After(50 * time.Millisecond):
		a.Terminate()
	}

	// When another agent process starts again
	a = infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.AgentDir = agentDir
	})
	a.RegisterPlugin(plugin1)
	go a.Run()

	// The full delta is submitted
	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}
	fixture.AssertRequestContainsInventoryDeltas(t, req, []*inventoryapi.RawDelta{
		{
			Source:   "test/dummy",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"dummy": map[string]interface{}{
					"value": "hello",
				},
			},
		},
	})
}

func TestDeltas_HarvestAfterStoreCleanup(t *testing.T) {
	const timeout = 5 * time.Second

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("metadata/attributes", 1),
		ihttp.ResetDeltasResponse("test/dummy"))

	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.CustomAttributes = config.CustomAttributeMap{
			"some":      "attr",
			"someother": "other_attr",
		}
		cfg.Verbose = 1
	})
	go a.Terminate()

	plugin := newDummyPlugin("hi", a.Context)
	a.RegisterPlugin(plugin)
	// That runs a reconnectable plugin (e.g. Custom Attributes plugin)
	a.RegisterPlugin(plugins.NewCustomAttrsPlugin(a.Context))
	go a.Run()

	plugin.ticker <- 1

	// That has successfully submitted data on start
	var req1 http.Request
	select {
	case req1 = <-testClient.RequestCh:
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}
	fixture.AssertRequestContainsInventoryDeltas(t, req1, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/attributes",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"customAttributes": map[string]interface{}{
					"some":      "attr",
					"someother": "other_attr",
				},
			},
		},
	})

	// When the server gets a reset all request
	plugin.value = "ho"
	plugin.ticker <- 1
	select {
	case _ = <-testClient.RequestCh:
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// The reconnectable plugins are run again and the removed inventory is resubmitted
	var req2 http.Request
	select {
	case req2 = <-testClient.RequestCh:
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}
	fixture.AssertRequestContainsInventoryDeltas(t, req2, []*inventoryapi.RawDelta{
		{
			Source:   "metadata/attributes",
			ID:       2,
			FullDiff: true,
			Diff: map[string]interface{}{
				"customAttributes": map[string]interface{}{
					"some":      "attr",
					"someother": "other_attr",
				},
			},
		},
	})
}

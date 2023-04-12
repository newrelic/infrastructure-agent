// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package core

import (
	"bytes"
	context2 "context"
	agentTypes "github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/inventory"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
)

type InventoryTestSuite struct {
	suite.Suite
	AsyncInventoryHandlerEnabled bool
}

func TestInventorySuite_AsyncInventoryHandlerEnabled(t *testing.T) {
	suite.Run(t, &InventoryTestSuite{
		AsyncInventoryHandlerEnabled: true,
	})
}

func TestInventorySuite_AsyncInventoryHandlerDisabled(t *testing.T) {
	suite.Run(t, &InventoryTestSuite{})
}

func (s *InventoryTestSuite) TestDeltas_nestedObjectsV4() {
	t := s.T()
	const timeout = 5 * time.Second

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("test/dummy", 1),
		ihttp.AcceptedResponse("test/dummy", 2))
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
	})

	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	// That runs a v4 plugin with nested inventory
	plugin := newDummyV4Plugin(t, `{
  "protocol_version": "4",
  "integration": {
    "name": "com.newrelic.foo",
    "version": "0.1.0"
  },
  "data": [
    {
      "inventory": {
        "foo": {
          "bar": {
            "baz": {
              "k1": "v1",
              "k2": false
            }
          }
        }
      }
    }
  ]
}`, a.Context)
	a.RegisterPlugin(plugin)

	go a.Run()

	// When the plugin harvests inventory data
	plugin.harvest()

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
			Source:   "test/dummy_v4",
			ID:       1,
			FullDiff: true,
			Diff: map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"baz": map[string]interface{}{
							"k1": "v1",
							"k2": false,
						},
					},
				},
			},
		},
	})
}

func (s *InventoryTestSuite) TestDeltas_BasicWorkflow() {
	t := s.T()

	const timeout = 5 * time.Second

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("test/dummy", 1),
		ihttp.AcceptedResponse("test/dummy", 2))
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
	})

	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	// That runs a plugin
	plugin := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin)

	go a.Run()

	// When the plugin harvests inventory data
	plugin.harvest()

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
	plugin.harvest()

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
	plugin.harvest()

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

func (s *InventoryTestSuite) TestDeltas_ForwardOnly() {
	t := s.T()

	const timeout = 5 * time.Second

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("test/dummy", 1),
		ihttp.AcceptedResponse("test/dummy", 2))
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
		config.IsForwardOnly = true
		config.FirstReapInterval = time.Nanosecond
		config.SendInterval = time.Nanosecond
	})

	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	//Give time to at least send one request
	ctxTimeout, _ := context2.WithTimeout(a.Context.Ctx, time.Millisecond*10)
	a.Context.Ctx = ctxTimeout

	// That runs a plugin
	plugin := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin)

	go a.Run()

	// When the plugin harvests inventory data
	plugin.harvest()

	select {
	case <-testClient.RequestCh:
		a.Terminate()
		assert.FailNow(t, "Agent must not send data yet")
	case <-ctxTimeout.Done():
		// Success
		return
	case <-time.After(timeout):
		a.Terminate()
	}
}

func (s *InventoryTestSuite) TestDeltas_ResendIfFailure() {
	t := s.T()
	const timeout = 5 * time.Second

	// Given an agent that fails submitting the deltas in the second invocation
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("test/dummy", 1),
		ihttp.ErrorResponse,
		ihttp.AcceptedResponse("test/dummy", 2))

	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
	})

	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	// That runs a plugin
	plugin := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin)

	go a.Run()

	// When the plugin harvests inventory data
	plugin.harvest()

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
	plugin.harvest()

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

func (s *InventoryTestSuite) TestDeltas_ResendAfterReset() {
	t := s.T()

	const timeout = 10 * time.Second

	agentDir, err := ioutil.TempDir("", "prefix")
	if err != nil {
		panic(err)
	}

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.SendInterval = time.Hour
		config.AgentDir = agentDir
		config.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
	})
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	// That runs a plugin
	plugin1 := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin1)

	go a.Run()

	// When the plugin harvests inventory data
	plugin1.harvest()

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
		config.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
	})
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})
	a.RegisterPlugin(plugin1)
	go a.Run()

	// The full delta is submitted
	var req http.Request
	select {
	case req = <-testClient.RequestCh:
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
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}
	a.Terminate()
}

func (s *InventoryTestSuite) TestDeltas_HarvestAfterStoreCleanup() {
	t := s.T()

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
		cfg.Log.Level = config.LogLevelDebug
		cfg.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
	})
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	go a.Terminate()

	plugin := newDummyPlugin("hi", a.Context)
	a.RegisterPlugin(plugin)
	// That runs a re-connectable plugin (e.g. Custom Attributes plugin)
	a.RegisterPlugin(plugins.NewCustomAttrsPlugin(a.Context))
	go a.Run()

	plugin.harvest()

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
	plugin.harvest()
	select {
	case _ = <-testClient.RequestCh:
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}

	// The re-connectable plugins are run again and the removed inventory is resubmitted
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

func (s *InventoryTestSuite) TestDeltas_UpdateIDLookupTable() {
	t := s.T()

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("metadata/attributes", 1))

	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.CustomAttributes = config.CustomAttributeMap{
			"some":      "attr",
			"someother": "other_attr",
		}
		cfg.Log.Level = config.LogLevelDebug
		cfg.AsyncInventoryHandlerEnabled = s.AsyncInventoryHandlerEnabled
	})

	go a.Run()
	defer a.Terminate()
	assert.Equal(t, "display-name", a.Context.EntityKey())

	dataset := agentTypes.PluginInventoryDataset{}
	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  "hostName.com",
		Source: sysinfo.HOST_SOURCE_HOSTNAME,
	})
	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  "instanceId",
		Source: sysinfo.HOST_SOURCE_INSTANCE_ID,
	})
	dataset = append(dataset, sysinfo.HostAliases{
		Alias:  "hostName",
		Source: sysinfo.HOST_SOURCE_HOSTNAME_SHORT,
	})
	a.Context.SendData(agentTypes.NewPluginOutput(ids.PluginID{Category: "metadata", Term: "host_aliases"}, entity.NewWithoutID("test"), dataset))

	assert.Equal(t, "instanceId", a.Context.EntityKey())
}

func BenchmarkInventoryProcessingPipeline(b *testing.B) {
	const timeout = 5 * time.Second

	// Given an agent
	testClient := ihttp.NewRequestRecorderClient(
		ihttp.AcceptedResponse("test/dummy", 1),
		ihttp.AcceptedResponse("test/dummy", 2))
	a := infra.NewAgent(testClient.Client)
	a.Context.SetAgentIdentity(entity.Identity{10, "abcdef"})

	// That runs a plugin
	plugin := newDummyPlugin("hello", a.Context)
	a.RegisterPlugin(plugin)

	go a.Run()

	// When the plugin harvests inventory data
	b.StartTimer()
	plugin.harvest()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		b.StopTimer()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(b, "timeout while waiting for a response")
	}

	// The full delta is submitted
	fixture.AssertRequestContainsInventoryDeltas(b, req, []*inventoryapi.RawDelta{
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

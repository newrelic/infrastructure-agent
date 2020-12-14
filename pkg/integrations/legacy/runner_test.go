// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package legacy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/test/fixture/integration"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"

	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
)

type RunnerSuite struct{}

var (
	_ = Suite(&RunnerSuite{})

	// Payload sample from public doc:
	// https://docs.newrelic.com/docs/integrations/integrations-sdk/file-specifications/integration-executable-file-specifications
	v1Payload = []byte(`{
		"name": "com.myorg.nginx",
		"protocol_version": "1",
		"integration_version": "1.0.0",
		"metrics": [{
			"event_type": "MyorgNginxSample",
			"net.connectionsActive": 54,
			"net.requestsPerSecond": 21,
			"net.connectionsReading": 23
		}],
		"inventory": {
			"events/worker_connections": {
				"value": 1024
			},
			"http/gzip": {
				"value": "on"
			}
		},
		"events": [{
			"summary": "More than 10 request errors logged in the last 5 minutes",
			"category": "notifications"
		}]
	}`)

	v2Payload = []byte(`{
	  "name": "com.newrelic.integration",
	  "protocol_version": "2",
	  "integration_version": "1.0.0-beta2",
	  "data": [
		{
		  "entity": {
			"name": "my_family_car",
			"type": "car"
		  },
		  "metrics": [
			{
			  "speed": 95,
			  "fuel": 768,
			  "passengers": 3,
			  "displayName": "my_family_car",
			  "entityName": "car:my_family_car",
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "renault", "cc": 1800}
		  },
		  "events": []
		},
		{
		  "entity": {
			"name": "street_hawk",
			"type": "motorbike"
		  },
		  "metrics": [
			{
			  "speed": 180,
			  "fuel": 128,
			  "passengers": 1,
			  "displayName": "street_hawk",
			  "entityName": "motorbike:street_hawk",
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "yamaha", "cc": 500}
		  },
		  "events": []
		}
	  ]
	}`)

	v3Payload = []byte(`{
	  "name": "com.newrelic.integration",
	  "protocol_version": "3",
	  "integration_version": "1.0.0-beta2",
	  "data": [
		{
		  "entity": {
			"name": "my_family_car",
			"type": "car",
			"id_attributes": [
				{
					"key": "env", 
					"value": "prod"
				},
				{
					"key": "srv", 
					"value": "auth"
				}
    		 ]
		  },
		  "metrics": [
			{
			  "speed": 95,
			  "fuel": 768,
			  "passengers": 3,
			  "displayName": "my_family_car",
			  "entityName": "car:my_family_car",
			  "reportingAgent": "reporting_agent_id",
			  "reportingEntityKey": "reporting_entity_key",
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "renault", "cc": 1800}
		  },
		  "events": []
		},
		{
		  "entity": {
			"name": "street_hawk",
			"type": "motorbike"
		  },
		  "metrics": [
			{
			  "speed": 180,
			  "fuel": 128,
			  "passengers": 1,
			  "displayName": "street_hawk",
			  "entityName": "motorbike:street_hawk",
			  "reportingEndpoint": "reporting_endpoint",
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "yamaha", "cc": 500}
		  },
		  "events": []
		}
	  ]
	}`)

	v2PayloadTestDisplayNameEntityName = []byte(`{
	  "name": "com.newrelic.integration",
	  "protocol_version": "2",
	  "integration_version": "1.0.0-beta2",
	  "data": [
		{
		  "entity": {
			"name": "my_family_car",
			"type": "car"
		  },
		  "metrics": [
			{
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "renault", "cc": 1800}
		  },
		  "events": []
		},
		{
		  "entity": {
			"name": "street_hawk",
			"type": "motorbike"
		  },
		  "metrics": [
			{
			  "displayName": "Family Motorbike",
			  "entityName": "Motorbike",
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "yamaha", "cc": 500}
		  },
		  "events": []
		},
		{
		  "metrics": [
			{
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "yamaha", "cc": 500}
		  },
		  "events": []
		}
	  ]
	}`)

	v2PayloadLocalEntity = []byte(`{
	  "name": "com.newrelic.integration",
	  "protocol_version": "2",
	  "integration_version": "1.0.0-beta2",
	  "data": [
		{
		  "metrics": [
			{
			  "speed": 95,
			  "fuel": 768,
			  "passengers": 3,
			  "displayName": "my_family_car",
			  "entityName": "car:my_family_car",
			  "event_type": "VehicleStatus"
			}
		  ],
		  "inventory": {
			"motor": {"brand": "renault", "cc": 1800}
		  },
		  "events": []
		}
	  ]
	}`)
	labelKeys = [2]string{"labels/role", "labels/environment"}
)

func generateTestDir() string {
	pluginDir := filepath.Join(os.TempDir(), "pluginTests")
	_ = os.RemoveAll(pluginDir)
	_ = os.MkdirAll(pluginDir, os.ModeDir|0755)
	return pluginDir
}

// Test that substituteProperties() handles a variety of scenarios, including properties whose names are a prefix to another property
func (rs *RunnerSuite) TestSubstituteProperties(c *C) {
	properties := map[string]string{
		"fooBar": "2",
		"baz":    "3",
		"foo":    "1",
	}

	c.Assert(substituteProperties("$fooBar", properties), Equals, "2")
	c.Assert(substituteProperties("$fooBar$foo", properties), Equals, "21")
	c.Assert(substituteProperties("$fooBaz", properties), Equals, "1Baz")
	c.Assert(substituteProperties("$baz$bar", properties), Equals, "3$bar")
}

// Test that newPluginInstance() produces the appropriate number of sources with property substitution working correctly in the data prefix path.
func (rs *RunnerSuite) TestNewPluginInstance(c *C) {
	plugin := &Plugin{
		Name: "foo",
		Sources: []*PluginSource{{
			Prefix: ids.PluginID{Category: "data", Term: "$prop1-data"},
		}, {
			Prefix: ids.PluginID{Category: "data", Term: "$prop1"},
		}},
		Properties: []PluginProperty{{
			Name: "prop1",
		}},
	}

	instance := newPluginInstance(plugin, map[string]string{
		"prop1": "bar",
	})

	c.Assert(len(instance.sources), Equals, 2)
	c.Assert(instance.sources[0].dataPrefix.String(), Equals, "data/bar-data")
	c.Assert(instance.sources[1].dataPrefix, Equals, ids.PluginID{Category: "data", Term: "bar"})
}

func (rs *RunnerSuite) TestCheckPrefixDuplicates(c *C) {
	runner := PluginRunner{
		instances: []*PluginInstance{{
			plugin: &Plugin{
				Name: "foo-bar",
			},
			sources: []*PluginSourceInstance{{
				dataPrefix: ids.PluginID{Category: "data", Term: "foo"},
			}, {
				dataPrefix: ids.PluginID{Category: "data", Term: "bar"},
			}},
		}},
	}

	newInstances := []*PluginInstance{{
		plugin: &Plugin{
			Name: "new-plugin",
		},
		sources: []*PluginSourceInstance{{
			dataPrefix: ids.PluginID{Category: "data", Term: "new-foo"},
		}, {
			dataPrefix: ids.PluginID{Category: "data", Term: "new-bar"},
		}},
	}}
	c.Assert(runner.checkPrefixDuplicates(newInstances), IsNil)

	newInstancesWithDups := []*PluginInstance{{
		plugin: &Plugin{
			Name: "new-plugin",
		},
		sources: []*PluginSourceInstance{{
			dataPrefix: ids.PluginID{Category: "data", Term: "new-foo"},
		}, {
			dataPrefix: ids.PluginID{Category: "data", Term: "bar"},
		}},
	}}
	if err := runner.checkPrefixDuplicates(newInstancesWithDups); err == nil {
		c.Error("No error produced when registering plugin instances with paths duplicated by existing instances in other plugins.")
	}

	newInstancesWithInternalDups := []*PluginInstance{{
		plugin: &Plugin{
			Name: "new-plugin",
		},
		sources: []*PluginSourceInstance{{
			dataPrefix: ids.PluginID{Category: "data", Term: "new-foo"},
		}, {
			dataPrefix: ids.PluginID{Category: "data", Term: "new-foo"},
		}},
	}}
	if err := runner.checkPrefixDuplicates(newInstancesWithInternalDups); err == nil {
		c.Error("No error produced when registering plugin instances with paths duplicated by existing instances in other plugins.")
	}
}

func (rs *RunnerSuite) TestRegisterInstances(c *C) {
	plugin := &Plugin{
		Name: "foo",
		Sources: []*PluginSource{
			{Prefix: ids.PluginID{Category: "data", Term: "$prop1-data"}, Command: []string{"foo", "$prop1"}},
			{Prefix: ids.PluginID{Category: "data", Term: "$prop1"}, Command: []string{"boo", "$prop1"}},
		},
		Properties: []PluginProperty{{
			Name: "prop1",
		}},
	}

	instance := newPluginInstance(plugin, map[string]string{
		"prop1": "bar",
	})

	ag := FakeAgent{
		Plugins: map[ids.PluginID]agent.Plugin{},
	}

	reg := &PluginRegistry{
		pluginInstances: []*PluginV1Instance{},
		plugins:         map[string]*Plugin{},
	}

	pInstances := []*PluginInstance{instance}

	runner := NewPluginRunner(reg, ag)
	runner.registerInstances(pInstances, make(chan string, 1000))
	c.Assert(len(ag.Plugins), Equals, 2)

	for _, source := range instance.sources {
		c.Assert(ag.Plugins[source.dataPrefix], Not(IsNil))
		plugin := ag.Plugins[source.dataPrefix].(*externalPlugin)
		c.Assert(plugin.pluginInstance.Name, Equals, instance.plugin.Name)
		c.Assert(plugin.pluginInstance.plugin, Equals, instance.plugin)
		c.Assert(reflect.DeepEqual(plugin.pluginCommand.Command, source.command), Equals, true)
		c.Assert(plugin.pluginCommand.Prefix, Equals, source.dataPrefix)
		c.Assert(plugin.pluginCommand.Interval, Equals, source.source.Interval)
	}
}

// customContext implements the AgentContext interface for testing purposes
// It only has two channels to read/write events and inventory data from plugins
type customContext struct {
	ch  chan agent.PluginOutput
	ev  chan sample.Event
	cfg *config.Config
}

func (cc customContext) Context() context.Context {
	return context.TODO()
}

func (cc customContext) HostnameResolver() hostname.Resolver {
	return newFixedHostnameResolver("foo.bar", "short")
}

func (cc customContext) ActiveEntitiesChannel() chan string {
	return make(chan string, 100)
}

func (cc customContext) EntityKey() string {
	return ""
}

func (cc customContext) CacheServicePids(source string, pidMap map[int]string) {}

func (cc customContext) Config() *config.Config {
	return cc.cfg
}

func (cc customContext) GetServiceForPid(pid int) (service string, ok bool) {
	return "", false
}

func (cc customContext) SendData(data agent.PluginOutput) {
	cc.ch <- data
}

func (cc customContext) SendEvent(event sample.Event, entityKey entity.Key) {
	cc.ev <- event
}

func (cc customContext) Unregister(id ids.PluginID) {}

func (cc customContext) Version() string {
	return ""
}

func (cc customContext) AddReconnecting(agent.Plugin) {}

func (cc customContext) Reconnect() {}

func (cc customContext) IDLookup() host.IDLookup {
	idLookupTable := make(host.IDLookup)
	idLookupTable[sysinfo.HOST_SOURCE_HOSTNAME_SHORT] = "short_hostname"
	return idLookupTable
}

func (cc customContext) Identity() entity.Identity {
	return entity.EmptyIdentity
}

func newContext() customContext {
	configStr := `
collector_url:  http://foo.bar
ignored_inventory:
    - files/config/stuff.bar
    - files/config/stuff.foo
license_key: abc123
custom_attributes:
    my_group:  test group
    agent_role:  test role
`

	f, _ := ioutil.TempFile("", "opsmatic_config_test")
	_, _ = f.WriteString(configStr)
	_ = f.Close()

	cfg, _ := config.LoadConfig(f.Name())

	return customContext{
		ch:  make(chan agent.PluginOutput),
		ev:  make(chan sample.Event),
		cfg: cfg,
	}
}

func newFakePluginWithContext(pluginVersion int) (customContext, externalPlugin) {
	ctx := newContext()
	plugin := newFakePlugin(ctx, pluginVersion)
	return ctx, plugin
}

func newFakePlugin(ctx customContext, pluginVersion int) externalPlugin {
	var plugin externalPlugin
	if pluginVersion >= 1 {
		plugin = externalPlugin{
			PluginCommon: agent.PluginCommon{
				ID:      ids.PluginID{Category: "data", Term: "foo"},
				Context: ctx,
			},
			pluginInstance: &PluginV1Instance{
				Labels: map[string]string{"role": "fileserver", "environment": "development"},
				plugin: &Plugin{
					Name:            "new-plugin",
					ProtocolVersion: pluginVersion,
				},
				Arguments: map[string]string{
					"PATH":       "Path should be replaced by path env var",
					"CUSTOM_ARG": "testValue",
				},
			},
			pluginRunner: &PluginRunner{
				registry:  &PluginRegistry{},
				closeWait: &sync.WaitGroup{},
			},
			pluginCommand: &PluginV1Command{
				Command: []string{"go", "run", "test.go"},
			},
			lock: &sync.RWMutex{},
		}
	} else {
		instance := PluginInstance{
			plugin: &Plugin{
				Name: "new-plugin",
			},
		}
		source := PluginSourceInstance{
			dataPrefix: ids.PluginID{Category: "data", Term: "new-foo"},
			source:     &PluginSource{},
		}

		plugin = externalPlugin{
			PluginCommon: agent.PluginCommon{
				ID:      source.dataPrefix,
				Context: ctx,
			},
			pluginInstance: &PluginV1Instance{
				Name:      instance.plugin.Name,
				Arguments: source.source.Env,
				plugin:    instance.plugin,
			},
			pluginCommand: &PluginV1Command{
				Command:  source.command,
				Prefix:   source.dataPrefix,
				Interval: source.source.Interval,
			},
			pluginRunner: &PluginRunner{
				registry:  &PluginRegistry{},
				closeWait: &sync.WaitGroup{},
			},
		}
	}
	plugin.logger = plugin.newLogger()
	return plugin
}

func newFakePluginWithEnvVars(pluginVersion int) externalPlugin {
	ctx := newContext()
	plugin := externalPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      ids.PluginID{Category: "data", Term: "foo"},
			Context: ctx,
		},
		pluginInstance: &PluginV1Instance{
			Labels: map[string]string{"role": "fileserver", "environment": "development"},
			plugin: &Plugin{
				Name:            "new-plugin",
				ProtocolVersion: pluginVersion,
			},
			Arguments: map[string]string{
				"PATH":       "Path should be replaced by path env var",
				"CUSTOM_ARG": "testValue",
				"PASSWORD1":  "pa$$word",
				"PASSWORD2":  "pa$tor",
				"PASSWORD3":  "pa${tor}",
				"PASSWORD4":  "pa$envVar",
				"PASSWORD5":  "pa${envVar}rest",
				"PASSWORD6":  "pa${otherEnvVar}something${envVar}rest",
			},
		},
		pluginRunner: &PluginRunner{
			registry:  &PluginRegistry{},
			closeWait: &sync.WaitGroup{},
		},
		pluginCommand: &PluginV1Command{
			Command: []string{"go", "run", "test.go"},
		},
		lock: &sync.RWMutex{},
	}
	plugin.logger = plugin.newLogger()
	return plugin
}

func readFromChannel(ch chan interface{}) (interface{}, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	select {
	case res := <-ch:
		return res, nil
	case <-ticker.C:
		return nil, errors.New("timeout reading channel output")
	}
}

func readData(ch chan agent.PluginOutput) (agent.PluginOutput, error) {
	var output agent.PluginOutput

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	select {
	case output = <-ch:
	case <-ticker.C:
		return output, errors.New("timeout reading channel output")
	}

	return output, nil
}

func readMetrics(ch chan sample.Event) (map[string]interface{}, error) {
	var event sample.Event

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	select {
	case event = <-ch:
	case <-ticker.C:
		return nil, errors.New("timeout reading channel output")
	}

	bs, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	var eventMap map[string]interface{}
	if err := json.Unmarshal(bs, &eventMap); err != nil {
		return nil, err
	}

	return eventMap, nil
}

func (rs *RunnerSuite) TestPluginHandleOutputV1(c *C) {
	ctx, plugin := newFakePluginWithContext(1)
	plugin.pluginInstance.IntegrationUser = "test"

	var outputJSON = `{"name": "test", "protocol_version" : "1", "integration_version": "1.0.0", ` +
		`"inventory":{"first": {"value": "fake", "nested": {"key": "value"}}}, ` +
		`"metrics": [{"event_type": "LoadBalancerSample", "id": "first", "value": "random"}]}`

	extraLabels := data.Map{}
	pipeReader, pipeWriter := io.Pipe()

	go plugin.handleOutput(pipeReader, extraLabels, []data.EntityRewrite{})
	_, _ = pipeWriter.Write([]byte(outputJSON))
	_ = pipeWriter.Close()

	rd, err := readData(ctx.ch)
	c.Assert(err, IsNil)

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)

	c.Assert(rd, NotNil)
	c.Assert(len(rd.Data), Equals, 4)
	c.Assert(rd.Data[0].SortKey(), Equals, "first")

	invData := rd.Data[3].(protocol.InventoryData)
	c.Assert(invData["id"], Equals, "integrationUser")
	c.Assert(invData["value"], Equals, "test")

	c.Assert(event, NotNil)
	c.Assert(event["event_type"], Equals, "LoadBalancerSample")
	c.Assert(event["id"], Equals, "first")
	c.Assert(event["value"], Equals, "random")
	c.Assert(event["integrationUser"], Equals, "test")

	for _, labelKey := range labelKeys {
		if rd.Data[1].SortKey() != labelKey && rd.Data[2].SortKey() != labelKey {
			c.Errorf("There isn't label '%s'' in the inventory", labelKey)
		}
	}
}

func (rs *RunnerSuite) TestPluginHandleOutputEventsV1(c *C) {
	ctx, plugin := newFakePluginWithContext(1)

	var outputJSON = `{"name": "test", "protocol_version" : "1", "integration_version": "1.0.0", ` +
		`"events": [{"summary": "hello world", "entity_name": "server", "format": "twisted", "custom_field": "custom_value", "attributes": {"attrKey": "attrValue", "category": "attrCategory", "summary": "attrSummary"}}]}`

	extraLabels := data.Map{
		"label.expected":     "extra label",
		"label.important":    "true",
		"special.annotation": "not a label but a fact",
	}
	pipeReader, pipeWriter := io.Pipe()
	go plugin.handleOutput(pipeReader, extraLabels, []data.EntityRewrite{})
	_, _ = pipeWriter.Write([]byte(outputJSON))
	_ = pipeWriter.Close()

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)

	c.Assert(event, NotNil)
	c.Assert(event["eventType"], Equals, "InfrastructureEvent")
	c.Assert(event["summary"], Equals, "hello world")
	c.Assert(event["category"], Equals, V1_DEFAULT_EVENT_CATEGORY)
	c.Assert(event["format"], Equals, "twisted")
	c.Assert(event["entity_name"], Equals, "server")
	c.Assert(event["custom_field"], Equals, "custom_value")

	// labels from pluginInstance
	c.Assert(event["label.environment"], Equals, "development")
	c.Assert(event["label.role"], Equals, "fileserver")

	// labels from databind
	c.Assert(event["label.expected"], Equals, "extra label")
	c.Assert(event["label.important"], Equals, "true")

	c.Assert(event["special.annotation"], Equals, nil)

	c.Assert(event["attrKey"], Equals, "attrValue")
	// To avoid collisions repeated attributes are namespaced
	c.Assert(event["attr.summary"], Equals, "attrSummary")
	c.Assert(event["attr.category"], Equals, "attrCategory")
}

func (rs *RunnerSuite) TestPluginHandleOutputEventsNoSummaryV1(c *C) {
	ctx, plugin := newFakePluginWithContext(1)

	var outputJSON = `{"name": "test", "protocol_version" : "1", "integration_version": "1.0.0", ` +
		`"events": [{"message": "hello world", "format": "twisted", "hack": "nothing"}]}`

	pipeReader, pipeWriter := io.Pipe()
	extraLabels := data.Map{}
	go plugin.handleOutput(pipeReader, extraLabels, []data.EntityRewrite{})
	_, _ = pipeWriter.Write([]byte(outputJSON))
	_ = pipeWriter.Close()

	event, err := readMetrics(ctx.ev)
	c.Assert(err, NotNil)

	c.Assert(event, IsNil)
}

func (rs *RunnerSuite) TestPluginHandleOutputV1NewEventTypes(c *C) {
	ctx, plugin := newFakePluginWithContext(1)

	var outputJSON = `{"name": "test", "protocol_version" : "1", "integration_version": "1.0.0", ` +
		`"metrics": [{"event_type": "MyEventType", "id": "first", "value": "random"}]}`

	pipeReader, pipeWriter := io.Pipe()

	extraLabels := data.Map{}
	go plugin.handleOutput(pipeReader, extraLabels, []data.EntityRewrite{})
	_, _ = pipeWriter.Write([]byte(outputJSON))
	_ = pipeWriter.Close()

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)
	c.Assert(event, NotNil)
}

func (rs *RunnerSuite) TestPluginHandleOutputV1IllformedEventTypes(c *C) {
	ctx, plugin := newFakePluginWithContext(1)

	var outputJSON = `{"name": "test", "protocol_version" : "1", "integration_version": "1.0.0", ` +
		`"metrics": [{"missing_event_type": "LoadBalancerSample", "id": "first", "value": "random"}]}`

	pipeReader, pipeWriter := io.Pipe()

	extraLabels := data.Map{}
	go plugin.handleOutput(pipeReader, extraLabels, []data.EntityRewrite{})
	_, _ = pipeWriter.Write([]byte(outputJSON))
	_ = pipeWriter.Close()

	event, err := readMetrics(ctx.ev)
	c.Assert(err, NotNil)
	c.Assert(event, IsNil)
}

type FakeAgent struct {
	Plugins map[ids.PluginID]agent.Plugin
}

func (a FakeAgent) RegisterPlugin(p agent.Plugin) {
	a.Plugins[p.Id()] = p
}

func (FakeAgent) GetContext() agent.AgentContext { return nil }

func (rs *RunnerSuite) TestEventsPluginRunV1(c *C) {
	if runtime.GOOS == "windows" {
		c.Skip("timeouts on windows CI")
	}

	ctx, plugin := newFakePluginWithContext(1)

	plugin.pluginCommand.Command = []string{
		os.Args[0],
		fmt.Sprintf("-check.f=TestRunHelperV1"),
	}
	plugin.pluginInstance.Arguments = map[string]string{"GO_WANT_HELPER_PROCESS": "1"}
	plugin.pluginInstance.plugin.ProtocolVersion = protocol.V1
	plugin.pluginRunner.agent = FakeAgent{
		Plugins: map[ids.PluginID]agent.Plugin{},
	}

	plugin.pluginRunner.closeWait.Add(1)

	go plugin.Run()

	rd, err := readData(ctx.ch)
	c.Assert(err, IsNil)

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)

	c.Assert(rd, NotNil)
	c.Assert(len(rd.Data), Equals, 3)
	c.Assert(rd.Data[0].SortKey(), Equals, "first")

	c.Assert(event, NotNil)
	c.Assert(event["event_type"], Equals, "LoadBalancerSample")
	c.Assert(event["id"], Equals, "first")
	c.Assert(event["value"], Equals, "random")

	for _, d := range rd.Data {
		item := d.(protocol.InventoryData)
		if item["id"].(string) == "label/role" {
			c.Assert(item["value"].(string), Equals, "fileserver")
		}
		if item["id"].(string) == "label/environment" {
			c.Assert(item["value"].(string), Equals, "development")
		}
	}
}

func TestEventsPluginRunV1OverloadingStderrBuffer(t *testing.T) {
	//if runtime.GOOS == "windows" {
	t.Skip("Temporarily disabled. This test works well in Linux but does not work with Windows configuration.")
	//}
	ctx, plugin := newFakePluginWithContext(1)
	g, err := filepath.Abs("fixtures/overflow/overflow.go")
	assert.NoError(t, err)

	plugin.pluginCommand.Command = []string{
		"go",
		"run",
		g,
	}

	plugin.pluginInstance.Arguments = map[string]string{"GO_WANT_HELPER_PROCESS": "1"}
	plugin.pluginInstance.plugin.ProtocolVersion = protocol.V1
	plugin.pluginRunner.agent = FakeAgent{
		Plugins: map[ids.PluginID]agent.Plugin{},
	}

	plugin.pluginRunner.closeWait.Add(1)

	// We add a hook on the current log in order to test logged messages.
	logHook := test.NewGlobal()
	go plugin.Run()

	event, err := readMetrics(ctx.ev)

	// No log messages
	assert.Len(t, logHook.Entries, 0)
	assert.NoError(t, err)

	assert.NotNil(t, event)
	assert.Equal(t, "OverflowTest", event["event_type"])
	assert.Equal(t, "ipsum", event["lorem"])
	assert.Equal(t, "sit", event["dolor"])

}

func (rs *RunnerSuite) TestEventsPluginRunCrash(c *C) {
	_, plugin := newFakePluginWithContext(1)

	plugin.pluginCommand.Command = []string{
		os.Args[0],
		fmt.Sprintf("-check.f=TestRunHelperCrash"),
	}

	// Solves a race condition with other tests where the plugin command is
	// not properly clean
	defer func() {
		plugin.lock.Lock()
		defer plugin.lock.Unlock()
		plugin.pluginCommand.Command = []string{}
	}()

	plugin.pluginInstance.Arguments = map[string]string{"GO_WANT_HELPER_PROCESS": "1"}
	plugin.pluginInstance.plugin.ProtocolVersion = protocol.V1

	plugin.pluginRunner.closeWait.Add(1)

	go plugin.Run()
}

func (rs *RunnerSuite) TestRunHelperV1(c *C) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	//defer os.Exit(0)

	var outputJSON = `{"name": "test", "protocol_version" : "1", "integration_version": "1.0.0", ` +
		`"inventory":{"first": {"value": "fake", "nested": {"key": "value"}}}, ` +
		`"metrics": [{"event_type": "LoadBalancerSample", "id": "first", "value": "random"}]}`
	fmt.Println(outputJSON)
}

func (rs *RunnerSuite) TestRunHelperCrash(c *C) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	_, _ = fmt.Fprintln(os.Stderr, "Random error in the plugin")
	os.Exit(1)
}

// Ensure that we get the expected reasonable error messages given a variety of bad protocol_versions in output
func (rs *RunnerSuite) TestBadProtocolVersions(c *C) {
	plugin := &externalPlugin{
		pluginInstance: &PluginV1Instance{
			Name: "test",
		},
		pluginCommand: &PluginV1Command{
			Prefix: ids.PluginID{"test", "prefix"},
		},
		logger: log.WithField("", ""),
	}

	ctx := new(mocks.AgentContext)
	cfg := &config.Config{
		ForceProtocolV2toV3: false,
	}
	ctx.On("Config").Return(cfg)
	ctx.On("EntityKey").Return("my-agent-id")
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("foo.bar", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	plugin.Context = ctx

	extraLabels := data.Map{}
	var entityRewrite []data.EntityRewrite
	ok, err := plugin.handleLine([]byte(`{}`), extraLabels, entityRewrite)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Equals, "protocol_version is not defined")
	c.Assert(ok, Equals, false)
	ok, err = plugin.handleLine([]byte(`{"protocol_version": "abc"}`), extraLabels, entityRewrite)
	c.Assert(err, DeepEquals, errors.New("Protocol version 'abc' could not be parsed as an integer."))
	c.Assert(ok, Equals, false)
	ok, err = plugin.handleLine([]byte(`{"protocol_version": "abc"}`), extraLabels, entityRewrite)
	c.Assert(err, DeepEquals, errors.New("Protocol version 'abc' could not be parsed as an integer."))
	c.Assert(ok, Equals, false)
	ok, err = plugin.handleLine([]byte(`{"protocol_version": 1.5}`), extraLabels, entityRewrite)
	c.Assert(err, DeepEquals, errors.New("Protocol version 1.5 was a float, not an integer."))
	c.Assert(ok, Equals, false)
	ok, err = plugin.handleLine([]byte(`{"protocol_version": "1500"}`), extraLabels, entityRewrite)
	c.Assert(err, DeepEquals, errors.New("unsupported protocol version: 1500. Please try updating the Agent to the newest version."))
	c.Assert(ok, Equals, false)
}

func (rs *RunnerSuite) TestHandleOutputV2GoldenPath(c *C) {
	ctx, plugin := newFakePluginWithContext(1)

	extraLabels := data.Map{
		"label.expected":     "extra label",
		"label.important":    "true",
		"special_annotation": "not a label but a fact",
		"another_annotation": "100% a fact",
	}
	go func() {
		ok, err := plugin.handleLine([]byte(`{
      "protocol_version": "2",
      "data": [{
        "metrics": [{
          "event_type": "TestEvent",
          "someValue": 10
        }]
      }]
    }`), extraLabels, []data.EntityRewrite{})
		c.Assert(err, IsNil)
		c.Assert(ok, Equals, true)
	}()

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)
	c.Assert(event, NotNil)
	c.Assert(event["event_type"], Equals, "TestEvent")
	c.Assert(event["someValue"], Equals, float64(10))

	// databind labels
	c.Assert(event["label.expected"], Equals, "extra label")
	c.Assert(event["label.important"], Equals, "true")

	// databind annotations
	c.Assert(event["special_annotation"], Equals, "not a label but a fact")
	c.Assert(event["another_annotation"], Equals, "100% a fact")
}

func (rs *RunnerSuite) TestHandleOutputV1(c *C) {
	ctx, plugin := newFakePluginWithContext(1)
	plugin.pluginInstance.IntegrationUser = "test"

	extraLabels := data.Map{}
	go func() {
		ok, err := plugin.handleLine(v1Payload, extraLabels, []data.EntityRewrite{})
		c.Assert(ok, Equals, true)
		c.Assert(err, IsNil)
	}()

	// data is expected to be read first, then metrics
	rd, err := readData(ctx.ch)
	c.Assert(err, IsNil)

	// labels are added as inventory
	c.Assert(len(rd.Data)-len(labelKeys), Equals, 3)

	firstData := rd.Data[0]
	inv := firstData.(protocol.InventoryData)
	// inventory order is not guaranteed
	if firstData.SortKey() == "events/worker_connections" {
		c.Assert(inv["id"], Equals, "events/worker_connections")
		c.Assert(inv["value"], Equals, float64(1024))
	} else if firstData.SortKey() == "http/gzip" {
		c.Assert(inv["id"], Equals, "http/gzip")
		c.Assert(inv["value"], Equals, "on")
	} else {
		c.Error("unexpected first sort key")
	}

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)
	c.Assert(event, NotNil)
	c.Assert(event["event_type"], Equals, "MyorgNginxSample")
	c.Assert(event["net.connectionsActive"], Equals, float64(54))
}

func (rs *RunnerSuite) TestHandleOutputV2WithLocalEntity(c *C) {
	ctx, plugin := newFakePluginWithContext(1)

	extraLabels := data.Map{}
	go func() {
		ok, err := plugin.handleLine(v2PayloadLocalEntity, extraLabels, []data.EntityRewrite{})
		c.Assert(err, IsNil)
		c.Assert(ok, Equals, true)
	}()

	rd, err := readData(ctx.ch)
	c.Assert(err, IsNil)
	c.Assert(rd.Data[0].SortKey(), Equals, "motor")
	c.Assert(rd.Data[0].(protocol.InventoryData)["cc"], Equals, float64(1800))
	c.Assert(rd.Data[0].(protocol.InventoryData)["brand"], Equals, "renault")

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)
	c.Assert(event, NotNil)
	c.Assert(event["event_type"], Equals, "VehicleStatus")
	c.Assert(event["speed"], Equals, float64(95))

	_, err = readData(ctx.ch)
	c.Assert(err, NotNil)

	_, err = readMetrics(ctx.ev)
	c.Assert(err, NotNil)
}

func (rs *RunnerSuite) TestHandleOutputV2(c *C) {
	ctx, plugin := newFakePluginWithContext(1)

	extraLabels := data.Map{}
	go func() {
		ok, err := plugin.handleLine(v2Payload, extraLabels, []data.EntityRewrite{})
		c.Assert(err, IsNil)
		c.Assert(ok, Equals, true)
	}()

	rd, err := readData(ctx.ch)
	c.Assert(err, IsNil)
	c.Assert(rd.Data[0].SortKey(), Equals, "motor")
	c.Assert(rd.Data[0].(protocol.InventoryData)["cc"], Equals, float64(1800))
	c.Assert(rd.Data[0].(protocol.InventoryData)["brand"], Equals, "renault")

	event, err := readMetrics(ctx.ev)
	c.Assert(err, IsNil)
	c.Assert(event, NotNil)
	c.Assert(event["event_type"], Equals, "VehicleStatus")
	c.Assert(event["speed"], Equals, float64(95))

	rd, err = readData(ctx.ch)
	c.Assert(err, IsNil)
	c.Assert(rd.Data[0].SortKey(), Equals, "motor")
	c.Assert(rd.Data[0].(protocol.InventoryData)["cc"], Equals, float64(500))
	c.Assert(rd.Data[0].(protocol.InventoryData)["brand"], Equals, "yamaha")

	event, err = readMetrics(ctx.ev)
	c.Assert(err, IsNil)
	c.Assert(event, NotNil)
	c.Assert(event["event_type"], Equals, "VehicleStatus")
	c.Assert(event["speed"], Equals, float64(180))

}

// SKIP: go might not be on the path"
//func (rs *RunnerSuite) TestGenerateExecCmd(c *C) {
//	_, plugin := newFakePluginWithContext(1)
//	plugin.updateCmdWrappers("")
//	cmd := plugin.getCmdWrappers()
//	c.Assert(cmd, HasLen, 1)
//
//	// This will check that the system resolves the command "go" to its absolute path based on the system's $PATH
//	goPath, err := exec.LookPath("go")
//	c.Assert(err, IsNil)
//	c.Assert(cmd[0].cmd.Path, Equals, goPath)
//	c.Assert(cmd[0].cmd.Args, DeepEquals, []string{"go", "run", "test.go"}) // Go puts the command back in the args, so we expect "go" to be here
//
//	// Convert the env back into a map for easier checking
//	envVarMap := make(map[string]string)
//	for _, envVar := range cmd[0].cmd.Env {
//		envVarParts := strings.Split(envVar, "=")
//		envVarMap[envVarParts[0]] = envVarParts[1]
//	}
//	c.Assert(envVarMap["PATH"], Equals, os.Getenv("PATH")) // The config has a PATH set as well, but the env var should take precedence
//	c.Assert(envVarMap["VERBOSE"], Equals, "0")            // Should be set based on config
//	c.Assert(envVarMap["CUSTOM_ARG"], Equals, "testValue") // Should match the config on the plugin instance
//	// In config.go, these environment variables are also configured to pass through
//	if runtime.GOOS == "windows" {
//		c.Assert(envVarMap["ComSpec"], Equals, os.Getenv("ComSpec"))
//		c.Assert(envVarMap["SystemRoot"], Equals, os.Getenv("SystemRoot"))
//	}
//}

func TestGenerateExecCmdWithDatabind_FetchError(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	_, plugin := newFakePluginWithContext(1)
	mb := new(mockBinder)
	plugin.binder = mb
	src := &databind.Sources{}
	plugin.pluginInstance.plugin.discovery = src

	mb.On("Fetch", src).Return(databind.Values{}, errors.New("fetch error with password=secret"))
	plugin.updateCmdWrappers("")
	cmds := plugin.getCmdWrappers()
	assert.Empty(t, cmds)
	require.NotEmpty(t, hook.AllEntries())
	var found bool
	for i := range hook.AllEntries() {
		if hook.AllEntries()[i].Level == logrus.WarnLevel {
			if val, ok := hook.AllEntries()[i].Data["error"]; ok {
				if val.(error).Error() == "fetch error with password=<HIDDEN>" {
					found = true
				}
			}
		}
	}
	assert.True(t, found)
}

func TestGenerateExecCmdWithDatabind_ReplaceError(t *testing.T) {
	log.SetOutput(ioutil.Discard)  // discard logs so not to break race tests
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	_, plugin := newFakePluginWithContext(1)
	mb := new(mockBinder)
	plugin.binder = mb
	src := &databind.Sources{}
	plugin.pluginInstance.plugin.discovery = src

	vars := data.Map{}
	tmpVals := databind.NewValues(vars)
	mb.On("Fetch", src).Return(tmpVals, nil)

	mb.On("Replace", mock.AnythingOfType("*databind.Values"), mock.AnythingOfType("legacy.cfgTmp"), mock.AnythingOfType("[]databind.ReplaceOption")).
		Return([]data.Transformed{}, errors.New("replace error with password=secret"))

	plugin.updateCmdWrappers("")
	cmds := plugin.getCmdWrappers()
	assert.Empty(t, cmds)
	require.NotEmpty(t, hook.AllEntries())
	var found bool
	for i := range hook.AllEntries() {
		if hook.AllEntries()[i].Level == logrus.WarnLevel {
			if val, ok := hook.AllEntries()[i].Data["error"]; ok {
				if val.(error).Error() == "replace error with password=<HIDDEN>" {
					found = true
				}
			}
		}
	}
	assert.True(t, found)
}

func TestGenerateExecCmdWithDatabind(t *testing.T) {
	t.Skip("go might not be on the path")

	_, plugin := newFakePluginWithContext(1)
	mb := new(mockBinder)
	plugin.binder = mb
	src := &databind.Sources{}
	plugin.pluginInstance.plugin.discovery = src

	vars := data.Map{}
	tmpVals := databind.NewValues(vars)
	mb.On("Fetch", src).Return(tmpVals, nil)

	configs := []data.Transformed{
		{
			Variables: cfgTmp{
				CommandLine: []string{"go", "run", "test.go"},
				Environment: map[string]string{
					"PATH":       os.Getenv("PATH"), // The config has a PATH set as well, but the env var should take precedence.
					"VERBOSE":    "0",
					"CUSTOM_ARG": "testValue",
					"ComSpec":    os.Getenv("ComSpec"),
					"SystemRoot": os.Getenv("SystemRoot"),
				},
			},
			MetricAnnotations: data.Map{
				"Expected": "Label",
			},
			EntityRewrites: nil,
		},
	}
	mb.On("Replace", mock.AnythingOfType("*databind.Values"), mock.AnythingOfType("legacy.cfgTmp"), mock.AnythingOfType("[]databind.ReplaceOption")).
		Return(configs, nil)

	plugin.updateCmdWrappers("")
	cmd := plugin.getCmdWrappers()
	assert.Len(t, cmd, 1)

	// This will check that the system resolves the command "go" to its absolute path based on the system's $PATH
	goPath, err := exec.LookPath("go")
	require.NoError(t, err)
	assert.Equal(t, goPath, cmd[0].cmd.Path)
	assert.Equal(t, []string{"go", "run", "test.go"}, cmd[0].cmd.Args) // Go puts the command back in the args, so we expect "go" to be here

	// Convert the env back into a map for easier checking
	envVarMap := make(map[string]string)
	for _, envVar := range cmd[0].cmd.Env {
		envVarParts := strings.Split(envVar, "=")
		envVarMap[envVarParts[0]] = envVarParts[1]
	}
	assert.Equal(t, os.Getenv("PATH"), envVarMap["PATH"]) // The config has a PATH set as well, but the env var should take precedence
	assert.Equal(t, "0", envVarMap["VERBOSE"])            // Should be set based on config
	assert.Equal(t, "testValue", envVarMap["CUSTOM_ARG"]) // Should match the config on the plugin instance

	// In config.go, these environment variables are also configured to pass through
	if runtime.GOOS == "windows" {
		assert.Equal(t, os.Getenv("ComSpec"), envVarMap["ComSpec"], Equals)
		assert.Equal(t, os.Getenv("SystemRoot"), envVarMap["SystemRoot"], Equals)
	}
}

// Check that when given a relative executable which exists inside the plugin directory, we use the absolute path to it
func (rs *RunnerSuite) TestGenerateExecCmdRelativePath(c *C) {
	pluginDir := filepath.Join(generateTestDir(), "TestGenerateExecCmdRelativePath")
	c.Assert(os.MkdirAll(pluginDir, os.ModeDir|0755), IsNil)
	_, err := os.OpenFile(filepath.Join(pluginDir, "TestExecutable"), os.O_RDONLY|os.O_CREATE, 0755)
	c.Assert(err, IsNil)

	_, plugin := newFakePluginWithContext(1)
	plugin.pluginCommand.Command[0] = "TestExecutable"
	plugin.updateCmdWrappers(pluginDir)
	cmd := plugin.getCmdWrappers()
	c.Assert(cmd, HasLen, 1)

	c.Assert(cmd[0].cmd.Path, Equals, filepath.Join(pluginDir, "TestExecutable"))
}

// Check that when given a relative executable which exists inside the plugin directory, with a ./ prefix, we use the
// absolute path to it
func (rs *RunnerSuite) TestGenerateExecCmdRelativePathWithDotSlash(c *C) {
	// Given a folder
	pluginDir := filepath.Join(generateTestDir(), "TestGenerateExecCmdRelativePath")
	c.Assert(os.MkdirAll(pluginDir, os.ModeDir|0755), IsNil)

	// With a plugin inside
	_, err := os.OpenFile(filepath.Join(pluginDir, "TestExecutable"), os.O_RDONLY|os.O_CREATE, 0755)
	c.Assert(err, IsNil)

	// When providing an executable command that is not located in the plugins dir
	_, plugin := newFakePluginWithContext(1)
	plugin.pluginCommand.Command[0] = "./TestExecutable"

	plugin.updateCmdWrappers(pluginDir)
	cmd := plugin.getCmdWrappers()
	c.Assert(cmd, HasLen, 1)
	// Then the path to the command is exactly the absolute path
	c.Assert(cmd[0].cmd.Path, Equals, filepath.Join(pluginDir, "TestExecutable"))
}

// Check that when given a relative executable which exists inside the plugin directory, given its subfolder, we use the
// absolute path to it
func (rs *RunnerSuite) TestGenerateExecCmdRelativePathWithFolder(c *C) {
	// Given a folder
	pluginDir := filepath.Join(generateTestDir(), "TestGenerateExecCmdRelativePath")
	c.Assert(os.MkdirAll(pluginDir, os.ModeDir|0755), IsNil)

	// With a plugin inside, in a subfolder
	executableDir := filepath.Join(pluginDir, "bin")
	c.Assert(os.MkdirAll(executableDir, os.ModeDir|0755), IsNil)
	_, err := os.OpenFile(filepath.Join(executableDir, "TestExecutable"), os.O_RDONLY|os.O_CREATE, 0755)
	c.Assert(err, IsNil)

	// When providing an executable command that is not located in the plugins dir
	_, plugin := newFakePluginWithContext(1)
	plugin.pluginCommand.Command[0] = "bin/TestExecutable"

	plugin.updateCmdWrappers(pluginDir)
	cmd := plugin.getCmdWrappers()
	c.Assert(cmd, HasLen, 1)

	// Then the path to the command is exactly the absolute path
	c.Assert(cmd[0].cmd.Path, Equals, filepath.Join(executableDir, "TestExecutable"))
}

// Check that when given a relative executable which exists inside the plugin directory, given its subfolder wit a ./
// prefix, we use the absolute path to it
func (rs *RunnerSuite) TestGenerateExecCmdRelativePathWithFolderAndDotSlash(c *C) {
	// Given a folder
	pluginDir := filepath.Join(generateTestDir(), "TestGenerateExecCmdRelativePath")
	c.Assert(os.MkdirAll(pluginDir, os.ModeDir|0755), IsNil)

	// With a plugin inside, in a subfolder
	executableDir := filepath.Join(pluginDir, "bin")
	c.Assert(os.MkdirAll(executableDir, os.ModeDir|0755), IsNil)
	_, err := os.OpenFile(filepath.Join(executableDir, "TestExecutable"), os.O_RDONLY|os.O_CREATE, 0755)
	c.Assert(err, IsNil)

	// When providing an executable command that is not located in the plugins dir
	_, plugin := newFakePluginWithContext(1)
	plugin.pluginCommand.Command[0] = "./bin/TestExecutable"

	plugin.updateCmdWrappers(pluginDir)
	cmd := plugin.getCmdWrappers()
	c.Assert(cmd, HasLen, 1)

	// Then the path to the command is exactly the absolute path
	c.Assert(cmd[0].cmd.Path, Equals, filepath.Join(executableDir, "TestExecutable"))
}

// Check that when given an absolute path for a plugin, it is not converted to relative path
func (rs *RunnerSuite) TestGenerateExecCmdAbsolutePath(c *C) {
	// Given a folder
	pluginDir := filepath.Join(generateTestDir(), "TestGenerateExecCmdRelativePath")
	c.Assert(os.MkdirAll(pluginDir, os.ModeDir|0755), IsNil)

	// With a plugin inside
	_, err := os.OpenFile(filepath.Join(pluginDir, "TestExecutable"), os.O_RDONLY|os.O_CREATE, 0755)
	c.Assert(err, IsNil)

	// When providing an absolute path to such plugin when generating the executable command
	_, plugin := newFakePluginWithContext(1)
	plugin.pluginCommand.Command[0] = filepath.Join(pluginDir, "TestExecutable")

	plugin.updateCmdWrappers(pluginDir)
	cmd := plugin.getCmdWrappers()
	c.Assert(cmd, HasLen, 1)

	// Then the path to the command is exactly the absolute path
	c.Assert(cmd[0].cmd.Path, Equals, filepath.Join(pluginDir, "TestExecutable"))
}

// Check that when given an relative path for a plugin which is not found in the plugin path, it is not attached to
// the plugins dir (relying on the environment's PATH)
func (rs *RunnerSuite) TestGenerateExecCmdEnvironmentPath(c *C) {
	// Given a folder
	pluginDir := filepath.Join(generateTestDir(), "TestGenerateExecCmdRelativePath")
	c.Assert(os.MkdirAll(pluginDir, os.ModeDir|0755), IsNil)

	// When providing an executable command that is not located in the plugins dir
	_, plugin := newFakePluginWithContext(1)
	plugin.pluginCommand.Command[0] = "TestExecutableOnPath"

	plugin.updateCmdWrappers(pluginDir)
	cmd := plugin.getCmdWrappers()

	c.Assert(cmd, HasLen, 1)

	// Then the path to the command is exactly the absolute path
	c.Assert(cmd[0].cmd.Path, Equals, "TestExecutableOnPath")
}

// Check that we can expand environment variables if they exist but keep the original value if not
func (rs *RunnerSuite) TestGenerateExecWithEnvVars(c *C) {
	// given
	_ = os.Setenv("envVar", "test")
	_ = os.Setenv("otherEnvVar", "bob")
	defer func() {
		_ = os.Unsetenv("envVar")
		_ = os.Unsetenv("otherEnvVar")
	}()

	plugin := newFakePluginWithEnvVars(1)
	plugin.updateCmdWrappers("")
	cmd := plugin.getCmdWrappers()
	c.Assert(cmd, HasLen, 1)

	// This will check that the system resolves the command "go" to its absolute path based on the system's $PATH
	goPath, err := exec.LookPath("go")
	c.Assert(err, IsNil)
	c.Assert(cmd[0].cmd.Path, Equals, goPath)
	c.Assert(cmd[0].cmd.Args, DeepEquals, []string{"go", "run", "test.go"}) // Go puts the command back in the args, so we expect "go" to be here

	// Convert the env back into a map for easier checking
	envVarMap := make(map[string]string)
	for _, envVar := range cmd[0].cmd.Env {
		envVarParts := strings.Split(envVar, "=")
		envVarMap[envVarParts[0]] = envVarParts[1]
	}
	c.Assert(envVarMap["PATH"], Equals, os.Getenv("PATH"))             // The config has a PATH set as well, but the env var should take precedence
	c.Assert(envVarMap["VERBOSE"], Equals, "0")                        // Should be set based on config
	c.Assert(envVarMap["CUSTOM_ARG"], Equals, "testValue")             // Should match the config on the plugin instance
	c.Assert(envVarMap["PASSWORD1"], Equals, "pa$$word")               // should be kept
	c.Assert(envVarMap["PASSWORD2"], Equals, "pa$tor")                 // should be kept
	c.Assert(envVarMap["PASSWORD3"], Equals, "pa${tor}")               // should be kept
	c.Assert(envVarMap["PASSWORD4"], Equals, "patest")                 // should be replaced
	c.Assert(envVarMap["PASSWORD5"], Equals, "patestrest")             // should be replaced
	c.Assert(envVarMap["PASSWORD6"], Equals, "pabobsomethingtestrest") // should be replaced

	// In config.go, these environment variables are also configured to pass through
	if runtime.GOOS == "windows" {
		c.Assert(envVarMap["ComSpec"], Equals, os.Getenv("ComSpec"))
		c.Assert(envVarMap["SystemRoot"], Equals, os.Getenv("SystemRoot"))
	}
}

func (rs *RunnerSuite) TestParsePayloadV2(c *C) {
	ctx := new(mocks.AgentContext)
	cfg := &config.Config{
		ForceProtocolV2toV3: false,
	}
	ctx.On("Config").Return(cfg)
	rd, version, err := ParsePayload(v2Payload, false)

	// Plugin output identifier
	c.Assert(err, IsNil)
	c.Assert(rd.PluginOutputIdentifier.Name, Equals, "com.newrelic.integration")

	pv, _ := protocol.VersionFromPayload(v2Payload, false)
	c.Assert(pv, Equals, 2)
	c.Assert(rd.PluginOutputIdentifier.IntegrationVersion, Equals, "1.0.0-beta2")

	// first entity
	c.Assert(rd.DataSets[0].Entity.Name, Equals, "my_family_car")
	c.Assert(rd.DataSets[0].Entity.Type, Equals, entity.Type("car"))
	c.Assert(rd.DataSets[0].Metrics[0]["speed"], Equals, float64(95))
	c.Assert(rd.DataSets[0].Metrics[0]["fuel"], Equals, float64(768))
	c.Assert(rd.DataSets[0].Metrics[0]["passengers"], Equals, float64(3))
	c.Assert(rd.DataSets[0].Metrics[0]["displayName"], Equals, "my_family_car")
	c.Assert(rd.DataSets[0].Metrics[0]["entityName"], Equals, "car:my_family_car")
	c.Assert(rd.DataSets[0].Metrics[0]["event_type"], Equals, "VehicleStatus")
	c.Assert(rd.DataSets[0].Inventory["motor"]["brand"], Equals, "renault")
	c.Assert(rd.DataSets[0].Inventory["motor"]["cc"], Equals, float64(1800))

	// second entity
	c.Assert(rd.DataSets[1].Entity.Name, Equals, "street_hawk")
	c.Assert(rd.DataSets[1].Entity.Type, Equals, entity.Type("motorbike"))
	c.Assert(rd.DataSets[1].Metrics[0]["speed"], Equals, float64(180))
	c.Assert(rd.DataSets[1].Metrics[0]["fuel"], Equals, float64(128))
	c.Assert(rd.DataSets[1].Metrics[0]["passengers"], Equals, float64(1))
	c.Assert(rd.DataSets[1].Metrics[0]["displayName"], Equals, "street_hawk")
	c.Assert(rd.DataSets[1].Metrics[0]["entityName"], Equals, "motorbike:street_hawk")
	c.Assert(rd.DataSets[1].Metrics[0]["event_type"], Equals, "VehicleStatus")
	c.Assert(rd.DataSets[1].Inventory["motor"]["brand"], Equals, "yamaha")
	c.Assert(rd.DataSets[1].Inventory["motor"]["cc"], Equals, float64(500))

	c.Assert(version, Equals, protocol.V2)
}

func TestParsePayloadV3(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := &config.Config{
		ForceProtocolV2toV3: false,
	}
	ctx.On("Config").Return(cfg)

	rd, version, err := ParsePayload(v3Payload, false)
	assert.NoError(t, err)
	assert.Equal(t, version, protocol.V3)

	// first entity
	assert.Equal(t, rd.DataSets[0].Entity.Name, "my_family_car")
	assert.Equal(t, rd.DataSets[0].Entity.Type, entity.Type("car"))
	assert.Equal(t, rd.DataSets[0].Entity.IDAttributes, entity.IDAttributes{
		{
			Key:   "env",
			Value: "prod",
		},
		{
			Key:   "srv",
			Value: "auth",
		},
	})
	assert.Equal(t, rd.DataSets[0].Metrics[0]["speed"], float64(95))
	assert.Equal(t, rd.DataSets[0].Metrics[0]["fuel"], float64(768))
	assert.Equal(t, rd.DataSets[0].Metrics[0]["passengers"], float64(3))
	assert.Equal(t, rd.DataSets[0].Metrics[0]["displayName"], "my_family_car")
	assert.Equal(t, rd.DataSets[0].Metrics[0]["entityName"], "car:my_family_car")
	assert.Equal(t, rd.DataSets[0].Metrics[0]["event_type"], "VehicleStatus")
	assert.Equal(t, rd.DataSets[0].Metrics[0]["reportingAgent"], "reporting_agent_id")
	assert.Equal(t, rd.DataSets[0].Metrics[0]["reportingEntityKey"], "reporting_entity_key")

	assert.Equal(t, rd.DataSets[0].Inventory["motor"]["brand"], "renault")
	assert.Equal(t, rd.DataSets[0].Inventory["motor"]["cc"], float64(1800))

	// second entity
	assert.Equal(t, rd.DataSets[1].Entity.Name, "street_hawk")
	assert.Equal(t, rd.DataSets[1].Entity.Type, entity.Type("motorbike"))
	assert.Equal(t, rd.DataSets[1].Metrics[0]["speed"], float64(180))
	assert.Equal(t, rd.DataSets[1].Metrics[0]["fuel"], float64(128))
	assert.Equal(t, rd.DataSets[1].Metrics[0]["passengers"], float64(1))
	assert.Equal(t, rd.DataSets[1].Metrics[0]["displayName"], "street_hawk")
	assert.Equal(t, rd.DataSets[1].Metrics[0]["entityName"], "motorbike:street_hawk")
	assert.Equal(t, rd.DataSets[1].Metrics[0]["event_type"], "VehicleStatus")
	assert.Equal(t, rd.DataSets[1].Metrics[0]["reportingEndpoint"], "reporting_endpoint")
	assert.Equal(t, rd.DataSets[1].Inventory["motor"]["brand"], "yamaha")
	assert.Equal(t, rd.DataSets[1].Inventory["motor"]["cc"], float64(500))
}

type fakeEmitter struct {
	lastEventData map[string]interface{}
	lastEntityKey string
}

func (f *fakeEmitter) EmitInventoryWithPluginId(data agent.PluginInventoryDataset, entityKey string, pluginId ids.PluginID) {
}

func (f *fakeEmitter) EmitInventory(data agent.PluginInventoryDataset, entity entity.Entity) {}

func (f *fakeEmitter) EmitEvent(eventData map[string]interface{}, entityKey entity.Key) {
	f.lastEventData = eventData
	f.lastEntityKey = string(entityKey)
}

func TestEmitPayloadV2NoDisplayNameNoEntityName(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := &config.Config{
		ForceProtocolV2toV3: false,
	}
	ctx.On("Config").Return(cfg)
	ctx.On("EntityKey").Return("my-agent-id")
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("foo.bar", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	rd, version, err := ParsePayload(v2PayloadTestDisplayNameEntityName, false)
	assert.NoError(t, err)
	extraAnnotations := map[string]string{}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite

	// Remote entity, No displayName, No entityName
	emitter := fakeEmitter{}
	assert.NoError(t, EmitDataSet(ctx, &emitter, "test/test", "x.y.z", "testuser", rd.DataSets[0], extraAnnotations, labels, entityRewrite, version))
	assert.EqualValues(t, "car:my_family_car", emitter.lastEventData["displayName"])
	assert.EqualValues(t, "car:my_family_car", emitter.lastEventData["entityName"])
	assert.EqualValues(t, "car:my_family_car", emitter.lastEventData["entityKey"])

	// Remote entity, with displayName, with entityName
	assert.NoError(t, EmitDataSet(ctx, &emitter, "test/test", "x.y.z", "testuser", rd.DataSets[1], extraAnnotations, labels, entityRewrite, version))
	assert.EqualValues(t, "Family Motorbike", emitter.lastEventData["displayName"])
	assert.EqualValues(t, "Motorbike", emitter.lastEventData["entityName"])
	assert.EqualValues(t, "motorbike:street_hawk", emitter.lastEventData["entityKey"])

	// Local entity, no displayName, no entityName
	assert.NoError(t, EmitDataSet(ctx, &emitter, "test/test", "x.y.z", "testuser", rd.DataSets[2], extraAnnotations, labels, entityRewrite, version))
	_, ok := emitter.lastEventData["displayName"]
	assert.False(t, ok)
	_, ok = emitter.lastEventData["entityName"]
	assert.False(t, ok)
	// but entityKey is the agent key
	assert.EqualValues(t, "my-agent-id", emitter.lastEventData["entityKey"])
}

func TestEmitDataSet_OnAddHostnameDecoratesWithHostname(t *testing.T) {
	evType := "foo"
	user := "user"
	hn := "longHostName"
	agentIdentifier := "my-agent-id"
	pluginName := "plugin/name"
	pluginVersion := "x.y.z"

	// no constructor as natural input is deserialization
	eFields := entity.Fields{
		Name: hn,
		Type: "baz",
	}

	d := protocol.PluginDataSetV3{
		PluginDataSet: protocol.PluginDataSet{
			Entity:      eFields,
			AddHostname: true,
			Metrics: []protocol.MetricData{
				{
					"event_type": evType,
				},
			},
		},
	}

	ctx := new(mocks.AgentContext)
	ctx.On("EntityKey").Return(agentIdentifier)
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver(hn, "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	em := &fakeEmitter{}
	extraAnnotations := map[string]string{}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite
	assert.NoError(t, EmitDataSet(ctx, em, pluginName, pluginVersion, user, d, extraAnnotations, labels, entityRewrite, protocol.V2))

	key, err := eFields.Key()
	assert.NoError(t, err)

	expectedDecoration := map[string]interface{}{
		"event_type":         evType,
		"eventType":          evType,
		"integrationName":    pluginName,
		"integrationVersion": pluginVersion,
		"integrationUser":    user,
		"entityKey":          key,
		"entityName":         key,
		"displayName":        key,
		"reportingAgent":     agentIdentifier,
	}

	assert.Equal(t, expectedDecoration, em.lastEventData)
	assert.Nil(t, expectedDecoration["hostname"])
}

func TestEmitDataSet_EntityNameLocalhostIsNotReplacedWithHostnameV2(t *testing.T) {
	evType := "foo"
	agID := "my-agent-id"
	user := "user"
	pluginName := "plugin/name"
	pluginVersion := "x.y.z"

	d := protocol.PluginDataSetV3{
		PluginDataSet: protocol.PluginDataSet{
			Entity: entity.Fields{
				Name: "localhost:666",
				Type: "baz",
			},
			Metrics: []protocol.MetricData{
				{
					"event_type": evType,
				},
			},
		},
	}

	ctx := new(mocks.AgentContext)
	ctx.On("EntityKey").Return(agID)
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("foo.bar", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	em := &fakeEmitter{}
	extraAnnotations := map[string]string{}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite
	assert.NoError(t, EmitDataSet(ctx, em, pluginName, pluginVersion, user, d, extraAnnotations, labels, entityRewrite, protocol.V2))

	expectedDecoration := map[string]interface{}{
		"event_type":         evType,
		"eventType":          evType,
		"integrationName":    pluginName,
		"integrationVersion": pluginVersion,
		"integrationUser":    user,
		"entityKey":          entity.Key("baz:localhost:666"),
		"entityName":         entity.Key("baz:localhost:666"),
		"displayName":        entity.Key("baz:localhost:666"),
		"reportingAgent":     agID,
	}

	assert.Equal(t, expectedDecoration, em.lastEventData)
}

func TestEmitDataSet_EntityNameLocalhostIsReplacedWithHostnameV3(t *testing.T) {
	evType := "foo"
	agID := "my-agent-id"
	user := "user"
	pluginName := "plugin/name"
	pluginVersion := "x.y.z"

	d := protocol.PluginDataSetV3{
		PluginDataSet: protocol.PluginDataSet{
			Entity: entity.Fields{
				Name: "localhost:666",
				Type: "baz",
			},
			Metrics: []protocol.MetricData{
				{
					"event_type": evType,
				},
			},
		},
	}

	ctx := new(mocks.AgentContext)
	ctx.On("EntityKey").Return(agID)
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("foo.bar", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	em := &fakeEmitter{}
	extraAnnotations := map[string]string{}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite
	assert.NoError(t, EmitDataSet(ctx, em, pluginName, pluginVersion, user, d, extraAnnotations, labels, entityRewrite, protocol.V3))

	expectedDecoration := map[string]interface{}{
		"event_type":         evType,
		"eventType":          evType,
		"integrationName":    pluginName,
		"integrationVersion": pluginVersion,
		"integrationUser":    user,
		"entityKey":          entity.Key("baz:display_name:666"),
		"entityName":         entity.Key("baz:display_name:666"),
		"displayName":        entity.Key("baz:display_name:666"),
		"reportingAgent":     agID,
	}

	assert.Equal(t, expectedDecoration, em.lastEventData)
}

func TestEmitDataSet_MetricHostnameIsReplacedIfLocalhostV3(t *testing.T) {
	evType := "foo"
	agID := "my-agent-id"
	user := "user"
	pluginName := "plugin/name"
	pluginVersion := "x.y.z"

	d := protocol.PluginDataSetV3{
		PluginDataSet: protocol.PluginDataSet{
			Entity: entity.Fields{
				Name: "localhost:666",
				Type: "baz",
			},
			Metrics: []protocol.MetricData{
				{
					"event_type": evType,
					"hostname":   "localhost",
				},
			},
		},
	}

	ctx := new(mocks.AgentContext)
	ctx.On("EntityKey").Return(agID)
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("foo.bar", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	em := &fakeEmitter{}
	extraAnnotations := map[string]string{}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite
	assert.NoError(t, EmitDataSet(ctx, em, pluginName, pluginVersion, user, d, extraAnnotations, labels, entityRewrite, protocol.V3))

	expectedDecoration := map[string]interface{}{
		"event_type":         evType,
		"eventType":          evType,
		"integrationName":    pluginName,
		"integrationVersion": pluginVersion,
		"integrationUser":    user,
		"entityKey":          entity.Key("baz:display_name:666"),
		"entityName":         entity.Key("baz:display_name:666"),
		"displayName":        entity.Key("baz:display_name:666"),
		"reportingAgent":     agID,
	}

	assert.EqualValues(t, expectedDecoration, em.lastEventData)
}

func TestEmitDataSet_ReportingFieldsAreReplacedIfLocalhostV3(t *testing.T) {
	agID := "my-agent-id"
	user := "user"
	pluginName := "plugin/name"
	pluginVersion := "x.y.z"

	d := protocol.PluginDataSetV3{
		PluginDataSet: protocol.PluginDataSet{
			Entity: entity.Fields{
				Name: "different_hostname:5555",
				Type: "baz",
			},
			Metrics: []protocol.MetricData{
				{
					"event_type":         "baz",
					"hostname":           "different_hostname",
					"reportingEndpoint":  "localhost:5555",
					"reportingEntityKey": "foo:localhost:5555",
				},
			},
		},
	}

	ctx := new(mocks.AgentContext)
	ctx.On("EntityKey").Return(agID)
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("foo.bar", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	em := &fakeEmitter{}
	extraAnnotations := map[string]string{}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite
	assert.NoError(t, EmitDataSet(ctx, em, pluginName, pluginVersion, user, d, extraAnnotations, labels, entityRewrite, protocol.V3))

	expectedDecoration := map[string]interface{}{
		"event_type":         "baz",
		"eventType":          "baz",
		"integrationName":    pluginName,
		"integrationVersion": pluginVersion,
		"integrationUser":    user,
		"entityKey":          entity.Key("baz:different_hostname:5555"),
		"entityName":         entity.Key("baz:different_hostname:5555"),
		"displayName":        entity.Key("baz:different_hostname:5555"),
		"reportingEndpoint":  "display_name:5555",
		"reportingEntityKey": "foo:display_name:5555",
		"reportingAgent":     agID,
	}

	assert.Equal(t, expectedDecoration, em.lastEventData)
}

func TestEmitDataSet_LogsEntityViolationsOncePerEntity(t *testing.T) {
	agID := "my-agent-id"
	user := "user"
	pluginVersion := "x.y.z"

	d := protocol.PluginDataSetV3{
		PluginDataSet: protocol.PluginDataSet{
			Entity: entity.Fields{
				Name: "foo",
				Type: "bar",
			},
			Metrics: []protocol.MetricData{
				{"event_type": "baz"},
			},
		},
	}
	for i := 0; i < maxEntityAttributeCount+1; i++ {
		d.Metrics[0][fmt.Sprintf("metric-%d", i)] = "foo"
	}

	ctx := new(mocks.AgentContext)
	ctx.On("EntityKey").Return(agID)
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("foo.bar", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())

	em := &fakeEmitter{}
	extraAnnotations := map[string]string{}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite

	// store log entries
	var w bytes.Buffer
	log.SetOutput(&w)
	assert.NoError(t, EmitDataSet(ctx, em, "plugin/name", pluginVersion, user, d, extraAnnotations, labels, entityRewrite, protocol.V3))
	assert.NoError(t, EmitDataSet(ctx, em, "plugin/name", pluginVersion, user, d, extraAnnotations, labels, entityRewrite, protocol.V3))
	assert.Equal(t, 1, strings.Count(w.String(), entityMetricsLengthWarnMgs))
}

func TestEmitDataSet_DoNotOverrideExistingMetrics(t *testing.T) {
	pluginVersion := "x.y.z"

	d := protocol.PluginDataSetV3{
		PluginDataSet: protocol.PluginDataSet{
			Entity: entity.Fields{
				Name: "foo",
				Type: "bar",
			},
			AddHostname: true,
			Metrics: []protocol.MetricData{
				{
					"event_type":   "FooSample",
					"cluster_name": "MyReportedCluster",
				},
			},
		},
	}

	ctx := new(mocks.AgentContext)
	ctx.On("EntityKey").Return("agent-id")
	ctx.On("HostnameResolver").Return(newFixedHostnameResolver("long", "short"))
	ctx.On("IDLookup").Return(newFixedIDLookup())
	em := &fakeEmitter{}
	extraAnnotations := map[string]string{
		"cluster_name":       "K8sDiscoveredCluster",
		"another_annotation": "hello",
	}
	labels := map[string]string{}
	var entityRewrite []data.EntityRewrite
	assert.NoError(t, EmitDataSet(ctx, em, "plugin/name", pluginVersion, "root", d, extraAnnotations, labels, entityRewrite, protocol.V3))

	expectedFields := map[string]interface{}{
		"event_type":         "FooSample",
		"eventType":          "FooSample",
		"cluster_name":       "MyReportedCluster",
		"another_annotation": "hello",
	}
	for k, v := range expectedFields {
		assert.Contains(t, em.lastEventData, k)
		assert.Equal(t, v, em.lastEventData[k])
	}
}

func TestProtocolV3_EntityKeyWithAttributes(t *testing.T) {
	t.Log("Protocol V3: entity key created with attributes when they are defined")
	{
		ctx, plugin := newFakePluginWithContext(1)
		extraLabels := data.Map{}
		go func() {
			ok, err := plugin.handleLine([]byte(`
{  
   "protocol_version":"3",
   "data":[  
      {  
         "entity":  
         {  
             "name":"104.24.42.1:80",
             "type":"instance",
			 "id_attributes" : [
				{
					"key": "env", 
					"value": "prod"
				},
				{
					"key": "srv", 
					"value": "auth"
				}
    		 ]
         },
         "metrics":[  
            {  
               "event_type":"TestEvent",
               "hostname":"104.24.42.1"
            }
         ]
      }
   ]
}
			`), extraLabels, []data.EntityRewrite{})
			assert.NoError(t, err)
			assert.True(t, ok)
		}()
		event, err := readMetrics(ctx.ev)
		assert.NoError(t, err)
		assert.Equal(t, "instance:104.24.42.1:80:env=prod:srv=auth", event["entityKey"])
		assert.Nil(t, event["hostname"])
	}
}

func TestProtocolV3(t *testing.T) {
	t.Log("Protocol V3: Add replacement of localhost (or its IP variations) with the agent harvested long hostname")
	var wg sync.WaitGroup
	wg.Add(1)
	{
		ctx, plugin := newFakePluginWithContext(1)
		extraLabels := data.Map{
			"label.expected":     "extra label",
			"label.important":    "true",
			"special_annotation": "not a label but a fact",
			"another_annotation": "100% a fact",
		}
		go func() {
			ok, err := plugin.handleLine([]byte(`
{  
   "protocol_version":"3",
   "data":[  
      {  
         "entity":  
         {  
             "name":"localhost:80",
             "type":"instance",
			 "id_attributes" : [
				{
					"key": "env", 
					"value": "prod"
				},
				{
					"key": "srv", 
					"value": "auth"
				}
    		 ]
         },
         "metrics":[  
            {  
               "event_type":"TestEvent",
               "some_metric": 100
            }
         ]
      }
   ]
}
			`), extraLabels, []data.EntityRewrite{})
			assert.NoError(t, err)
			assert.True(t, ok)
			wg.Done()
		}()
		event, err := readMetrics(ctx.ev)
		assert.NoError(t, err)
		assert.Equal(t, "instance:short_hostname:80:env=prod:srv=auth", event["entityKey"])
		// by default json numbers get converted to float64.
		assert.Equal(t, float64(100), event["some_metric"])
		assert.Equal(t, "extra label", event["label.expected"])
		assert.Equal(t, "true", event["label.important"])
		assert.Equal(t, "not a label but a fact", event["special_annotation"])
		assert.Equal(t, "100% a fact", event["another_annotation"])
		wg.Wait()
	}
}

func TestProtocolV2_EntityRewrite(t *testing.T) {
	t.Log("Protocol V2: EntityRewrite")
	{
		ctx, plugin := newFakePluginWithContext(1)
		extraLabels := data.Map{}
		entityRewrite := []data.EntityRewrite{
			{
				Action:       "replace",
				Match:        "0.0.0.0",
				ReplaceField: "container:abc",
			},
		}
		go func() {
			ok, err := plugin.handleLine([]byte(`
{  
   "protocol_version":"2",
   "data":[  
      {  
         "entity":  
         {  
             "name":"0.0.0.0:80",
             "type":"server"
         },
         "metrics":[  
            {  
               "event_type": "TestEvent",
               "hostname": "localhost"
            }
         ]
      }
   ]
}
			`), extraLabels, entityRewrite)
			assert.NoError(t, err)
			assert.True(t, ok)
		}()
		event, err := readMetrics(ctx.ev)
		assert.NoError(t, err)
		assert.Equal(t, "server:container:abc:80", event["entityKey"])
		assert.Nil(t, event["hostname"])
	}
}

func TestProtocolV2_LocalhostIsNotReplaced(t *testing.T) {
	t.Log("Protocol V2: Avoid localhost replacement")
	{
		ctx, plugin := newFakePluginWithContext(1)
		extraLabels := data.Map{}
		go func() {
			ok, err := plugin.handleLine([]byte(`
{  
   "protocol_version":"2",
   "data":[  
      {  
         "entity":  
         {  
             "name":"localhost:80",
             "type":"instance"
         },
         "metrics":[  
            {  
               "event_type": "TestEvent",
               "hostname": "localhost"
            }
         ]
      }
   ]
}
			`), extraLabels, []data.EntityRewrite{})
			assert.NoError(t, err)
			assert.True(t, ok)
		}()
		event, err := readMetrics(ctx.ev)
		assert.NoError(t, err)
		assert.Equal(t, "instance:localhost:80", event["entityKey"])
		assert.Nil(t, event["hostname"])
	}
}

type stubResolver struct {
	host string
}

func (s *stubResolver) Query() (full, short string, err error) {
	return s.host, s.host, nil
}

func (s *stubResolver) Long() string {
	return s.host
}

func BenchmarkEmitDataSet_MetricHostnameIsReplacedIfLocalhost(b *testing.B) {
	pluginVersion := "x.y.z"
	tests := []string{"localhost", "LOCALHOST", "127.0.0.1"}
	for _, t := range tests {
		b.Run(t, func(b *testing.B) {
			b.ReportAllocs()
			evType := "foo"
			agID := "my-agent-id"
			user := "user"

			d := protocol.PluginDataSetV3{
				PluginDataSet: protocol.PluginDataSet{
					Entity: entity.Fields{
						Name: fmt.Sprintf("%s:666", t),
						Type: "baz",
					},
					Metrics: []protocol.MetricData{
						{
							"event_type": evType,
							"hostname":   "127.0.0.1",
						},
					},
				},
			}

			ctx := new(mocks.AgentContext)
			ctx.On("EntityKey").Return(agID)
			ctx.On("IDLookup").Return(newFixedIDLookup())
			ctx.On("HostnameResolver").Return(&stubResolver{host: t})

			em := &fakeEmitter{}
			extraAnnotations := map[string]string{
				"label.expected":     "extra label",
				"label.important":    "true",
				"special.annotation": "not a label but a fact",
				"another.annotation": "100% a fact",
			}
			labels := map[string]string{}
			for i := 0; i < b.N; i++ {
				_ = EmitDataSet(ctx, em, "plugin/name", pluginVersion, user, d, extraAnnotations, labels, []data.EntityRewrite{}, protocol.V3)
			}
		})
	}
}

func TestLogFields(t *testing.T) {

	_, plugin := newFakePluginWithContext(3)

	// Overwrite the PATH argument with the env variable
	pathEnv := "my-path"
	origEnv := os.Getenv("PATH")
	_ = os.Setenv("PATH", pathEnv)
	defer func() {
		// not sure about this one. we probably shouldn't be changing a system env var...
		_ = os.Setenv("PATH", origEnv)
	}()

	fields := plugin.detailedLogFields()

	envVars := map[string]string{
		"VERBOSE":    "0",
		"PATH":       pathEnv,
		"CUSTOM_ARG": "testValue",
	}
	if runtime.GOOS == "windows" {
		require.Contains(t, fields, "env-vars")
		envVars["ComSpec"] = os.Getenv("ComSpec")
		envVars["SystemRoot"] = os.Getenv("SystemRoot")
	}

	assert.Equal(
		t,
		logrus.Fields{
			"integration":     "new-plugin",
			"instance":        "",
			"os":              "",
			"protocolVersion": 3,
			"workingDir":      "",
			"command":         "",
			"prefix":          ids.PluginID{Category: "", Term: ""},
			"interval":        0,
			"commandLine":     []string{"go", "run", "test.go"},
			"env-vars":        envVars,
			"arguments": map[string]string{
				"PATH":       "Path should be replaced by path env var",
				"CUSTOM_ARG": "testValue",
			},
			"labels": map[string]string{
				"role":        "fileserver",
				"environment": "development",
			},
		},
		fields,
	)
}

// Test for ParsePayloadV4 in different package
func TestParsePayload_v4WithV2ToV3UpgradeReturnsNoError(t *testing.T) {
	_, protocolV, err := ParsePayload(integration.ProtocolV4.Payload, true)
	assert.NoError(t, err)
	assert.Equal(t, protocol.V4, protocolV)
}

func newFixedHostnameResolver(long, short string) *fixedHostnameResolver {
	return &fixedHostnameResolver{
		long:  long,
		short: short,
	}
}

type fixedHostnameResolver struct {
	short string
	long  string
}

func (r *fixedHostnameResolver) Query() (full, short string, err error) {
	return r.long, r.short, nil
}

func (r *fixedHostnameResolver) Long() string {
	return r.long
}

func newFixedIDLookup() host.IDLookup {
	idLookupTable := make(host.IDLookup)
	idLookupTable[sysinfo.HOST_SOURCE_DISPLAY_NAME] = "display_name"
	return idLookupTable
}

type mockBinder struct {
	mock.Mock
}

func (m *mockBinder) Fetch(ctx *databind.Sources) (databind.Values, error) {
	args := m.Called(ctx)
	return args.Get(0).(databind.Values), args.Error(1)
}
func (m *mockBinder) Replace(vals *databind.Values, template interface{}, options ...databind.ReplaceOption) (transformedData []data.Transformed, err error) {
	args := m.Called(vals, template, options)
	return args.Get(0).([]data.Transformed), args.Error(1)
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/fixtures"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	config2 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	logHelper "github.com/newrelic/infrastructure-agent/test/log"
)

var terminatedQueue = make(chan string)

var passthroughEnv = []string{"GOCACHE", "GOPATH", "HOME", "PATH", "LOCALAPPDATA"}

func TestGroup_Run(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a grouprunner that runs two integrations
	te := &testemit.RecordEmitter{}
	loader := NewLoadFn(config2.YAML{
		Integrations: []config2.ConfigEntry{
			{InstanceName: "sayhello", Exec: testhelp.Command(fixtures.IntegrationScript, "hello"),
				Labels: map[string]string{"foo": "bar", "ou": "yea"}},
			{InstanceName: "saygoodbye", Exec: testhelp.Command(fixtures.IntegrationScript, "bye")},
		},
	}, nil)
	gr, _, err := NewGroup(loader, integration.InstancesLookup{}, nil, te, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, "", terminatedQueue, host.IDLookup{})
	require.NoError(t, err)

	// WHEN the Group executes all the integrations
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = gr.Run(ctx)

	// THEN the emitter eventually emits the metrics from all the integrations
	dataset, err := te.ReceiveFrom("sayhello")
	require.NoError(t, err)
	metrics := dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	assert.Equal(t, "TestSample", metrics[0]["event_type"])
	assert.Equal(t, "hello", metrics[0]["value"])
	assert.Equal(t, "bar", dataset.Metadata.Labels["foo"])
	assert.Equal(t, "yea", dataset.Metadata.Labels["ou"])

	dataset, err = te.ReceiveFrom("saygoodbye")
	require.NoError(t, err)
	metrics = dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	assert.Equal(t, "TestSample", metrics[0]["event_type"])
	assert.Equal(t, "bye", metrics[0]["value"])
	assert.Empty(t, dataset.Metadata.Labels)
}

func TestGroup_Run_Inventory(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a grouprunner that uses a Protocol 2 integration with inventory
	te := &testemit.RecordEmitter{}
	loader := NewLoadFn(config2.YAML{
		Integrations: []config2.ConfigEntry{
			{InstanceName: "nri-test", Exec: testhelp.GoRun(fixtures.InventoryGoFile, "key1=val1", "key2=val2"),
				Labels: map[string]string{"foo": "bar", "ou": "yea"}},
		},
	}, nil)
	gr, _, err := NewGroup(loader, integration.InstancesLookup{}, passthroughEnv, te, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, "", terminatedQueue, host.IDLookup{})
	require.NoError(t, err)

	// WHEN the integration is executed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = gr.Run(ctx)

	// THEN the metadata and entity data is emitted
	payload, err := te.ReceiveFrom("nri-test")
	require.NoError(t, err)
	data := payload.DataSet
	assert.Equal(t, "nri-test", payload.Metadata.Name)
	assert.Equal(t, "local-test", data.Cluster)
	assert.Equal(t, "test-service", data.Service)
	require.NoError(t, err)
	assert.Equal(t, "localtest", data.Entity.Name)
	assert.EqualValues(t, "test", data.Entity.Type)
	require.Len(t, data.Entity.IDAttributes, 1)
	assert.Equal(t, "idkey", data.Entity.IDAttributes[0].Key)
	assert.Equal(t, "idval", data.Entity.IDAttributes[0].Value)

	// AND the inventory is emitted
	assert.Empty(t, data.Metrics)
	assert.Empty(t, data.Events)
	require.Contains(t, data.Inventory, "cliargs")
	inventory := data.Inventory["cliargs"]
	assert.Equal(t, "val1", inventory["key1"])
	assert.Equal(t, "val2", inventory["key2"])

	// AND the labels are emitted
	assert.Equal(t, "bar", payload.Metadata.Labels["foo"])
	assert.Equal(t, "yea", payload.Metadata.Labels["ou"])

	// AND the default integration inventory should be set to empty
	assert.Equal(t, ids.EmptyInventorySource, payload.Metadata.InventorySource)
}

func TestGroup_Run_Inventory_OverridePrefix(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN an integration overriding the default inventory prefix
	te := &testemit.RecordEmitter{}
	loader := NewLoadFn(config2.YAML{
		Integrations: []config2.ConfigEntry{
			{InstanceName: "nri-test", Exec: testhelp.GoRun(fixtures.InventoryGoFile, "key1=val1"),
				InventorySource: "custom/inventory"},
		},
	}, nil)
	gr, _, err := NewGroup(loader, integration.InstancesLookup{}, passthroughEnv, te, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, "", terminatedQueue, host.IDLookup{})
	require.NoError(t, err)

	// WHEN the integration is executed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = gr.Run(ctx)

	// THEN the proper name and inventory are emitted
	payload, err := te.ReceiveFrom("nri-test")
	require.NoError(t, err)
	assert.Equal(t, "nri-test", payload.Metadata.Name)
	assert.Equal(t, "custom/inventory", payload.Metadata.InventorySource.String())
}

func TestGroup_Run_Timeout(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a grouprunner that runs an integration with a timeout
	te := &testemit.RecordEmitter{}
	to := 200 * time.Millisecond
	loader := NewLoadFn(config2.YAML{
		Integrations: []config2.ConfigEntry{
			{InstanceName: "Hello", Exec: testhelp.Command(fixtures.BlockedCmd), Timeout: &to},
		},
	}, nil)
	gr, _, err := NewGroup(loader, integration.InstancesLookup{}, nil, te, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, "", terminatedQueue, host.IDLookup{})
	require.NoError(t, err)
	errs := interceptGroupErrors(&gr)

	// WHEN the Group successfully executes the iteration and the timeout is reached
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = gr.Run(ctx)

	// THEN a "signal: killed" error is forwarded from the underlying command
	timeout := time.After(5 * time.Second)
	select {
	case err := <-errs:
		assert.NotNil(t, err)
	case <-timeout:
		assert.Fail(t, "expecting an error after killing the process because of the timeout")
	}
}

func TestGroup_Run_DiscoveryChangesUpdated(t *testing.T) {
	defer leaktest.Check(t)()

	discoveryCommand := testhelp.GoRun(fixtures.TimestampDiscovery)
	discoveryConf := `---
discovery:
  ttl: 0
  command:
    exec: ` + strings.Join(discoveryCommand, " ") + `
    match:
      timestamp: /./
`
	discovery, err := databind.LoadYAML([]byte(discoveryConf))
	require.NoError(t, err)

	// GIVEN a grouprunner that runs an integration with discovery configurations
	integr, err := integration.NewDefinition(config2.ConfigEntry{
		InstanceName: "timestamp",
		Exec:         testhelp.Command(fixtures.IntegrationScript, "${discovery.timestamp}"),
	}, integration.InstancesLookup{}, []string{}, nil)
	require.NoError(t, err)

	te := &testemit.RecordEmitter{}
	group := Group{
		emitter:      te,
		dSources:     discovery,
		integrations: []integration.Definition{integr},
		handleErrorsProvide: func() runnerErrorHandler {
			return func(_ context.Context, _ <-chan error) {}
		},
	}
	// shortening the interval to avoid long tests
	group.integrations[0].Interval = 100 * time.Millisecond

	// WHEN the integration is run repeated times
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = group.Run(ctx)

	// THEN the emitted value from discovery is different each time
	dataset, err := te.ReceiveFrom("timestamp")
	require.NoError(t, err)
	metrics := dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	require.Equal(t, "TestSample", metrics[0]["event_type"])

	firstValue := metrics[0]["value"]
	require.NotEmpty(t, firstValue)

	dataset, err = te.ReceiveFrom("timestamp")
	require.NoError(t, err)
	metrics = dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	require.Equal(t, "TestSample", metrics[0]["event_type"])
	secondValue := metrics[0]["value"]
	require.NotEmpty(t, secondValue)

	assert.NotEqual(t, firstValue, secondValue)
}

func TestGroup_Run_ConfigPathUpdated(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a grouprunner from an integration that embeds a config file
	te := &testemit.RecordEmitter{}
	loader := NewLoadFn(config2.YAML{
		Integrations: []config2.ConfigEntry{{
			InstanceName: "cfgpath",
			Exec:         testhelp.Command(fixtures.IntegrationScript, "${config.path}"),
			Config:       "hello",
		}},
	}, nil)
	group, _, err := NewGroup(loader, integration.InstancesLookup{}, nil, te, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, "", terminatedQueue, host.IDLookup{})
	require.NoError(t, err)
	// shortening the interval to avoid long tests
	group.integrations[0].Interval = 100 * time.Millisecond

	// WHEN the integration is run repeated times
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = group.Run(ctx)

	// THEN the emitted config path from discovery is not different each time
	dataset, err := te.ReceiveFrom("cfgpath")
	require.NoError(t, err)
	metrics := dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	require.Equal(t, "TestSample", metrics[0]["event_type"])

	firstValue := metrics[0]["value"]
	require.NotEmpty(t, firstValue)
	require.NotEqual(t, "${config.path}", firstValue)

	dataset, err = te.ReceiveFrom("cfgpath")
	require.NoError(t, err)
	metrics = dataset.DataSet.Metrics
	require.Len(t, metrics, 1)
	require.Equal(t, "TestSample", metrics[0]["event_type"])
	secondValue := metrics[0]["value"]
	require.NotEmpty(t, secondValue)
	require.NotEqual(t, "${config.path}", secondValue)

	assert.Equal(t, firstValue, secondValue)
}

func interceptGroupErrors(gr *Group) <-chan error {
	handledError := make(chan error, 1)
	gr.handleErrorsProvide = func() runnerErrorHandler {
		return func(_ context.Context, errs <-chan error) {
			handledError <- <-errs
		}
	}
	return handledError
}

func TestGroup_Run_IntegrationScriptPrintsErrorsAndReturnCodeIsZero(t *testing.T) {
	defer leaktest.Check(t)()

	// GIVEN a grouprunner that runs two integrations
	te := &testemit.RecordEmitter{}
	loader := NewLoadFn(config2.YAML{
		Integrations: []config2.ConfigEntry{
			{InstanceName: "log_errors", Exec: testhelp.Command(fixtures.IntegrationPrintsErr, "bye")},
		},
	}, nil)
	gr, _, err := NewGroup(loader, integration.InstancesLookup{}, nil, te, cmdrequest.NoopHandleFn, configrequest.NoopHandleFn, "", terminatedQueue, host.IDLookup{})
	require.NoError(t, err)

	// WHEN we add a hook to the log to capture the "error" and "fatal" levels
	hook := logHelper.NewInMemoryEntriesHook([]logrus.Level{logrus.FatalLevel, logrus.ErrorLevel})
	log.AddHook(hook)

	// WHEN the Group executes all the integrations
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = gr.Run(ctx)

	// THEN the emitter eventually emits the metrics from all the integrations
	_, err = te.ReceiveFrom("log_errors")
	require.NoError(t, err)

	// THEN we check the log entries from the hook
	want := []map[string]string{
		{
			"time":             `2020-02-11T17:28:50+01:00`,
			"level":            `error`,
			"msg":              `config: failed to sub lookup file data`,
			"component":        `integrations.runner.Group`,
			"error":            `config name: /var/db/something: p file, error: open /var/db/something: no such file or directory`,
			"integration_name": `nri-flex`,
		},
		{
			"time":             `2020-02-11T17:28:52+01:00`,
			"level":            `fatal`,
			"msg":              `config: fatal error`,
			"component":        `integrations.runner.Group`,
			"error":            `cannot read configuration file`,
			"integration_name": `nri-flex`,
		},
	}

	expLength := len(want)
	testhelpers.Eventually(t, 5*time.Second, func(rt require.TestingT) {
		entries := hook.GetEntries()
		actualLen := len(entries)
		assert.Equal(rt, actualLen, actualLen, "unexpected number of log entries")
		if expLength == actualLen {
			for i, w := range want {
				for k, v := range w {
					val, ok := entries[i].Data[k]
					assert.True(rt, ok)
					assert.Equal(rt, v, val)
				}
			}
		}
	})
}

func Test_parseLogrusFields(t *testing.T) {
	tests := map[string]struct {
		input string
		want  logrus.Fields
	}{
		"debug": {`time="2015-03-26T01:27:38-04:00" level=debug msg="Temperature changes" temperature=-4`, logrus.Fields{
			"time":        `2015-03-26T01:27:38-04:00`,
			"level":       `debug`,
			"msg":         `Temperature changes`,
			"temperature": `-4`,
		}},
		"info": {`time="2015-03-26T01:27:38-04:00" level=info msg="A group of walrus emerges from the ocean" animal=walrus size=10`, logrus.Fields{
			"time":   `2015-03-26T01:27:38-04:00`,
			"level":  `info`,
			"msg":    `A group of walrus emerges from the ocean`,
			"animal": `walrus`,
			"size":   `10`,
		}},
		"fatal": {`time="2015-03-26T01:27:38-04:00" level=fatal msg="The ice breaks!" err=&{0x2082280c0 map[animal:orca size:9009] 2015-03-26 01:27:38.441574009 -0400 EDT panic It's over 9000!} number=100 omg=true`, logrus.Fields{
			"time":   `2015-03-26T01:27:38-04:00`,
			"level":  `fatal`,
			"msg":    `The ice breaks!`,
			"err":    `&{0x2082280c0 map[animal:orca size:9009] 2015-03-26 01:27:38.441574009 -0400 EDT panic It's over 9000!}`,
			"number": `100`,
			"omg":    `true`,
		}},
		"with-escaped-quotes": {`time="2015-03-26T01:27:38-04:00" level=warning msg="The group's number \"increased\" tremendously!" number=122 omg=false`, logrus.Fields{
			"time":   `2015-03-26T01:27:38-04:00`,
			"level":  `warning`,
			"msg":    `The group's number \"increased\" tremendously!`,
			"number": `122`,
			"omg":    `false`,
		}},
		"flex-error": {`time="2020-02-11T17:28:50+01:00" level=error msg="config: failed to sub lookup file data" component=integrations.runner.Group error="config name: /var/db/something: p file, error: open /var/db/something: no such file or directory" integration_name=nri-flex`, logrus.Fields{
			"time":             `2020-02-11T17:28:50+01:00`,
			"level":            `error`,
			"msg":              `config: failed to sub lookup file data`,
			"component":        `integrations.runner.Group`,
			"error":            `config name: /var/db/something: p file, error: open /var/db/something: no such file or directory`,
			"integration_name": `nri-flex`,
		}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			out := parseLogrusFields(tc.input)

			for k, v := range tc.want {
				val, ok := out[k]
				assert.True(t, ok)
				assert.Equal(t, v, val)
			}
		})
	}
}

//nolint:paralleltest
func Test_parseSDKFields(t *testing.T) {
	tests := map[string]struct {
		input string
		want  logrus.Fields
	}{
		"debug": {`[DEBUG] Temperature changes`, logrus.Fields{
			"level": `debug`,
			"msg":   `Temperature changes`,
		}},
		"info": {`[INFO] A group of walrus emerges from the ocean`, logrus.Fields{
			"level": `info`,
			"msg":   `A group of walrus emerges from the ocean`,
		}},
		"fatal": {`[FATAL] The ice breaks!`, logrus.Fields{
			"level": `fatal`,
			"msg":   `The ice breaks!`,
		}},
		"error": {`[ERR] Error creating connection to SQL Server: lookup mssql on 192.168.65.5:53: no such host`, logrus.Fields{
			"level": `error`,
			"msg":   `Error creating connection to SQL Server: lookup mssql on 192.168.65.5:53: no such host`,
		}},
		"with-escaped-quotes": {`[WARN] The group's number \"increased\" tremendously!`, logrus.Fields{
			"level": `warn`,
			"msg":   `The group's number \"increased\" tremendously!`,
		}},

		"not matching": {`simple line`, nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			out := parseSDKFields(tc.input)

			for k, v := range tc.want {
				val, ok := out[k]
				assert.True(t, ok)
				assert.Equal(t, v, val)
			}
		})
	}
}

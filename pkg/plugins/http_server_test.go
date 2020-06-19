// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHTTPServerPlugin(t *testing.T) {

	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	var pOut *agent.PluginOutput = nil
	ctx.On("SendData", mock.Anything).Run(func(args mock.Arguments) {
		po := args.Get(0).(agent.PluginOutput)
		pOut = &po
	})
	ch := make(chan sample.Event, 10)
	ctx.On("SendEvent", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		ch <- args.Get(0).(sample.Event)
	})

	ctx.On("HostnameResolver").Return(testhelpers.NewFakeHostnameResolver("something.com", "sc", nil))
	ctx.On("AgentIdentifier").Return("test-agent")
	ctx.On("IDLookup").Return(agent.IDLookup{"test-agent": "test-agent-id"})

	// Given an HTTP Server Plugin
	port, err := testhelpers.GetFreePort()
	require.NoError(t, err)
	hsp := NewHTTPServerPlugin(ctx, "127.0.0.1", port)
	go hsp.Run()

	// that is listening
	client := http.Client{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for retries := 0; retries < 40; retries++ {
			_, err := client.Get(fmt.Sprintf("http://127.0.0.1:%v/some-path", port))
			if err == nil {
				wg.Done()
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		require.Fail(t, "can't get HTTPServerPlugin listening: %q", err.Error())
	}()

	wg.Wait()

	// When an integration sends data to the HTTP Server plugin
	body := strings.NewReader(`{"name":"int1","protocol_version":"1","integration_version":"1",` +
		`"metrics":[{"event_type":"MyMetric","value":123}],` +
		`"inventory":{"thing":{"urlEntry":"value"}},` +
		`"events":[{"summary":"blah","category":"bleh"}]}`)
	postReq, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%v/v1/data", port), body)
	resp, err := client.Do(postReq)
	require.NoError(t, err)

	// It returns 20x status
	require.Equal(t, 20, resp.StatusCode/10) // 200 OK or 204 No content

	events := make(map[string]map[string]interface{})
	for {
		se := <-ch
		sejson, _ := json.Marshal(se)
		var event map[string]interface{}
		err = json.Unmarshal(sejson, &event)
		require.NoError(t, err)
		events[event["eventType"].(string)] = event
		// InfrastructureEvent should be the last event sent
		if event["eventType"] == "InfrastructureEvent" {
			break
		}
	}
	// And the plugin inventory output is correctly emitted
	require.NotNil(t, pOut)
	require.Equal(t, "metadata/http_server", pOut.Id.String())
	inv := pOut.Data[0].(protocol.InventoryData)
	require.Equal(t, "value", inv["urlEntry"])
	require.Equal(t, "thing", inv["id"])

	// As well as the metrics and events
	require.Len(t, events, 2)
	require.Equal(t, "bleh", events["InfrastructureEvent"]["category"])
	require.Equal(t, "blah", events["InfrastructureEvent"]["summary"])
	require.Equal(t, "MyMetric", events["MyMetric"]["event_type"])
	require.InDelta(t, 123, events["MyMetric"]["value"], 0.01)
}

func newFixedIDLookup() agent.IDLookup {
	idLookupTable := make(agent.IDLookup)
	idLookupTable[sysinfo.HOST_SOURCE_DISPLAY_NAME] = "display_name"
	return idLookupTable
}

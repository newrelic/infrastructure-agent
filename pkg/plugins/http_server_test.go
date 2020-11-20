// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPServerPlugin(t *testing.T) {
	// Given an HTTP Server Plugin
	port, err := testhelpers.GetFreePort()
	require.NoError(t, err)

	e := &testemit.RecordEmitter{}

	hsp, err := NewHTTPServerPlugin(new(mocks.AgentContext), "127.0.0.1", port, e)
	require.NoError(t, err)

	go hsp.Run()

	// And an client connects
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

	// When a payload is posted to the HTTP endpoint
	body := strings.NewReader(`{"name":"int1","protocol_version":"1","integration_version":"1",` +
		`"metrics":[{"event_type":"MyMetric","value":123}],` +
		`"inventory":{"thing":{"urlEntry":"value"}},` +
		`"events":[{"summary":"blah","category":"bleh"}]}`)
	postReq, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%v/v1/data", port), body)
	resp, err := client.Do(postReq)
	require.NoError(t, err)

	// Then it returns 2XX
	require.Equal(t, 20, resp.StatusCode/10) // 200 OK or 204 No content

	// And posted data has been emitted
	d, err := e.ReceiveFrom(IntegrationName)
	assert.NoError(t, err)

	inv := d.DataSet.PluginDataSet.Inventory
	require.Len(t, inv, 1)
	assert.Equal(t, protocol.InventoryData{"urlEntry": "value"}, inv["thing"])

	metrics := d.DataSet.PluginDataSet.Metrics
	require.Len(t, metrics, 1)
	assert.Equal(t, "MyMetric", metrics[0]["event_type"])
	assert.Equal(t, float64(123), metrics[0]["value"])

	events := d.DataSet.PluginDataSet.Events
	require.Len(t, events, 1)
	assert.Equal(t, "blah", events[0]["summary"])
	assert.Equal(t, "bleh", events[0]["category"])
}

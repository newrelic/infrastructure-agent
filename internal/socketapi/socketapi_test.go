// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package socketapi

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/testhelp/testemit"
	network_helpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayloadFwServer_Serve(t *testing.T) {
	port, err := network_helpers.TCPPort()
	require.NoError(t, err)

	e := &testemit.RecordEmitter{}
	pf := NewServer(e, port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pf.Serve(ctx)

	payloadWritten := make(chan struct{})
	go func() {
		pf.WaitUntilReady()
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		require.NoError(t, err)
		_, err = conn.Write([]byte(strings.Replace(`{
  "protocol_version": "4",
  "integration": {
    "name": "com.newrelic.foo",
    "version": "0.1.0"
  },
  "data": [
    {
      "inventory": {
        "foo": {
          "k1": "v1",
          "k2": false
        }
      }
    }
  ]
}`, "\n", "", -1) + "\n"))
		assert.NoError(t, err)
		close(payloadWritten)
	}()

	select {
	case <-time.NewTimer(500 * time.Millisecond).C:
		t.Fail()
		return
	case <-payloadWritten:
	}

	d, err := e.ReceiveFrom(IntegrationName)
	assert.NoError(t, err)
	assert.NotEmpty(t, d)
}

var il = integration.InstancesLookup{
	Legacy: func(_ integration.DefinitionCommandConfig) (integration.Definition, error) {
		return integration.Definition{Name: "bar"}, nil
	},
	ByName: func(_ string) (string, error) {
		return "foo", nil
	},
}

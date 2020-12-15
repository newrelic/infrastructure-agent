// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runintegration

import (
	"context"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	dm "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm/testutils"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	l = log.WithComponent("test")
)

func TestHandle_returnsErrorOnMissingIntegrationName(t *testing.T) {
	h := NewHandler(make(chan integration.Definition, 1), integration.ErrLookup, dm.NewNoopEmitter(), l)

	cmdArgsMissingName := commandapi.Command{
		Args: []byte(`{ "integration_args": ["foo", "bar"] }`),
	}

	err := h.Handle(context.Background(), cmdArgsMissingName, false)
	assert.Equal(t, cmdchannel.NewArgsErr(ErrNoIntName).Error(), err.Error())
}

func TestHandle_queuesIntegrationToBeRun(t *testing.T) {
	defQueue := make(chan integration.Definition, 1)
	il := integration.InstancesLookup{
		Legacy: func(_ integration.DefinitionCommandConfig) (integration.Definition, error) {
			return integration.Definition{}, nil
		},
		ByName: func(_ string) (string, error) {
			return "/path/to/nri-foo", nil
		},
	}
	h := NewHandler(defQueue, il, dm.NewNoopEmitter(), l)

	cmd := commandapi.Command{
		Args: []byte(`{ "integration_name": "nri-foo", "integration_args": ["bar", "baz"] }`),
	}

	err := h.Handle(context.Background(), cmd, false)
	require.NoError(t, err)

	d := <-defQueue
	assert.Equal(t, "nri-foo", d.Name)
	// Definition won't allow assert further
}

func TestHandle_notifiesPlatform(t *testing.T) {
	defQueue := make(chan integration.Definition, 1)
	il := integration.InstancesLookup{
		Legacy: func(_ integration.DefinitionCommandConfig) (integration.Definition, error) {
			return integration.Definition{}, nil
		},
		ByName: func(_ string) (string, error) {
			return "/path/to/nri-foo", nil
		},
	}
	em := dm.NewRecordEmitter()
	h := NewHandler(defQueue, il, em, l)

	cmd := commandapi.Command{
		Args: []byte(`{ "integration_name": "nri-foo", "integration_args": ["bar", "baz"] }`),
		Metadata: map[string]interface{}{
			"meta key": "meta value",
		},
	}

	err := h.Handle(context.Background(), cmd, false)
	require.NoError(t, err)

	gotFRs := em.Received()
	require.Len(t, gotFRs, 1)
	require.Len(t, gotFRs[0].Data.DataSets, 1)
	gotEvents := gotFRs[0].Data.DataSets[0].Events
	require.Len(t, gotEvents, 1)
	expectedEvent := protocol.EventData{
		"eventType":             "InfrastructureEvent",
		"category":              "notifications",
		"summary":               "cmd-api",
		"cmd_name":              "run_integration",
		"cmd_hash":              "nri-foo#[bar baz]",
		"cmd_args_name":         "nri-foo",
		"cmd_args_args":         "[bar baz]",
		"cmd_metadata.meta key": "meta value",
	}
	assert.Equal(t, expectedEvent, gotEvents[0])
}

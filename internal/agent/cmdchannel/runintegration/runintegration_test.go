// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runintegration

import (
	"context"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	l = log.WithComponent("test")
)

func TestHandle_returnsErrorOnMissingIntegrationName(t *testing.T) {
	h := NewHandler(make(chan integration.Definition, 1), integration.ErrLookup, l)

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
	h := NewHandler(defQueue, il, l)

	cmd := commandapi.Command{
		Args: []byte(`{ "integration_name": "nri-foo", "integration_args": ["bar", "baz"] }`),
	}

	err := h.Handle(context.Background(), cmd, false)
	require.NoError(t, err)

	d := <-defQueue
	assert.Equal(t, "nri-foo", d.Name)
	// Definition won't allow assert further
}

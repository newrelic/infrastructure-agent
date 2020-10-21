// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runintegration

import (
	"context"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/stretchr/testify/assert"
)

func TestHandle_returnsErrorOnMissingIntegrationName(t *testing.T) {
	h := NewHandler()

	cmdArgsMissingName := commandapi.Command{
		Args: []byte(`{ "integration_args": ["foo", "bar"] }`),
	}

	_, err := h.Handle(context.Background(), cmdArgsMissingName, false)
	assert.Equal(t, cmdchannel.NewArgsErr(NoIntNameErr).Error(), err.Error())
}

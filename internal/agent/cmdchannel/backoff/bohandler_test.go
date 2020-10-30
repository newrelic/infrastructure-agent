// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package backoff

import (
	"context"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHandler(t *testing.T) {
	cmd := commandapi.Command{
		Name: "backoff_command_channel",
		Args: []byte(`{ "delay": 3000 }`),
	}

	boChan := make(chan int, 1)
	require.NoError(t, NewHandler(boChan).Handle(context.Background(), cmd, false))
	assert.Equal(t, 3000, <-boChan)
}

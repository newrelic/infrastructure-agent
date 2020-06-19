// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHeartBeatSampler(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})
	heartBeatSampler := NewHeartbeatSampler(ctx)

	sample1, err := heartBeatSampler.Sample()
	if err != nil {
		t.Fatal(err)
	}
	_, ok := sample1[0].(HeartbeatSample)

	assert.True(t, true, ok)
}

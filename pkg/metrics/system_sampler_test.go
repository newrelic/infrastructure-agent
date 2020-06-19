// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/stretchr/testify/assert"
)

func TestNewSystemSampler(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	m := NewSystemSampler(ctx, nil)

	assert.NotNil(t, m)
}

func TestSystemSample(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	storage := storage.NewSampler(ctx)
	m := NewSystemSampler(ctx, storage)

	result, err := m.Sample()

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

func BenchmarkSystem(b *testing.B) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{})

	storage := storage.NewSampler(ctx)
	m := NewSystemSampler(ctx, storage)
	for n := 0; n < b.N; n++ {
		m.Sample()
	}
}

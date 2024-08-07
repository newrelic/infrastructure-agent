// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package process

import (
	"errors"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProcessSampler_Sample(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Times(4).Return(cfg)

	harvester := &HarvesterMock{}
	sampler := NewProcessSampler(ctx).(*processSampler)
	sampler.harvest = harvester

	samples := []*types.ProcessSample{
		{
			ProcessDisplayName: "proc 1",
			ProcessID:          1,
		},
		{
			ProcessDisplayName: "proc 2",
			ProcessID:          2,
		},
		{
			ProcessDisplayName: "proc 3",
			ProcessID:          3,
		},
	}
	var pids []int32
	for _, s := range samples {
		pids = append(pids, s.ProcessID)
	}

	harvester.ShouldReturnPids(pids, nil)
	for _, s := range samples {
		harvester.ShouldDo(s.ProcessID, 0, s, nil)
	}

	eventBatch, err := sampler.Sample()
	assert.Nil(t, err)
	assert.Len(t, eventBatch, len(samples))
	for i, e := range eventBatch {
		assert.Equal(t, samples[i], e)
	}

	mock.AssertExpectationsForObjects(t, ctx, harvester)
}

func TestProcessSampler_Sample_ErrorOnProcessShouldNotStop(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Times(4).Return(cfg)

	harvester := &HarvesterMock{}
	sampler := NewProcessSampler(ctx).(*processSampler)
	sampler.harvest = harvester

	samples := []struct {
		pid  int32
		name string
		err  error
	}{
		{
			name: "proc 1",
			pid:  1,
		},
		{
			name: "proc 2",
			pid:  2,
			err:  errors.New("some error"),
		},
		{
			name: "proc 3",
			pid:  3,
		},
	}
	var pids []int32
	for _, s := range samples {
		pids = append(pids, s.pid)
	}

	harvester.ShouldReturnPids(pids, nil)
	for _, s := range samples {
		harvester.ShouldDo(s.pid, 0, &types.ProcessSample{ProcessID: s.pid, ProcessDisplayName: s.name}, s.err)
	}

	eventBatch, err := sampler.Sample()
	assert.Nil(t, err)
	assert.Len(t, eventBatch, 2)
	assert.Equal(t, int32(1), eventBatch[0].(*types.ProcessSample).ProcessID)
	assert.Equal(t, int32(3), eventBatch[1].(*types.ProcessSample).ProcessID)

	mock.AssertExpectationsForObjects(t, ctx, harvester)
}

func TestProcessSampler_Sample_DockerDecorator(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := &config.Config{RunMode: config.ModeRoot}
	ctx.On("Config").Times(4).Return(cfg)

	harvester := &HarvesterMock{}
	sampler := NewProcessSampler(ctx).(*processSampler)
	sampler.harvest = harvester
	sampler.containerSamplers = []metrics.ContainerSampler{&fakeContainerSampler{}}

	samples := []*types.ProcessSample{
		{
			ProcessDisplayName: "proc 1",
			ProcessID:          1,
		},
		{
			ProcessDisplayName: "proc 2",
			ProcessID:          2,
		},
		{
			ProcessDisplayName: "proc 3",
			ProcessID:          3,
		},
	}
	var pids []int32
	for _, s := range samples {
		pids = append(pids, s.ProcessID)
	}

	harvester.ShouldReturnPids(pids, nil)
	for _, s := range samples {
		harvester.ShouldDo(s.ProcessID, 0, s, nil)
	}

	eventBatch, err := sampler.Sample()
	assert.Nil(t, err)
	assert.Len(t, eventBatch, len(samples))
	for i, e := range eventBatch {
		flatProcessSample := e.(*types.FlatProcessSample)
		assert.Equal(t, float64(samples[i].ProcessID), (*flatProcessSample)["processId"])
		assert.Equal(t, samples[i].ProcessDisplayName, (*flatProcessSample)["processDisplayName"])
		assert.Equal(t, "decorated", (*flatProcessSample)["containerImage"])
		assert.Equal(t, "value1", (*flatProcessSample)["containerLabel_label1"])
		assert.Equal(t, "value2", (*flatProcessSample)["containerLabel_label2"])
	}

	mock.AssertExpectationsForObjects(t, ctx, harvester)
}

//nolint:paralleltest
func TestProcessSampler_Sample_DisabledDockerDecorator(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := config.NewConfig()
	cfg.ProcessContainerDecoration = false
	ctx.On("Config").Times(4).Return(cfg)

	// The container sampler getter should not be called
	containerSamplerGetter = func(cacheTTL time.Duration, dockerAPIVersion, dockerContainerdNamespace string) []metrics.ContainerSampler {
		t.Errorf("containerSamplerGetter should not be called")

		return nil
	}

	defer func() {
		containerSamplerGetter = metrics.GetContainerSamplers
	}()

	var expected []metrics.ContainerSampler
	sampler := NewProcessSampler(ctx).(*processSampler) //nolint:forcetypeassert
	assert.Equal(t, expected, sampler.containerSamplers)
}

//nolint:paralleltest
func TestProcessSampler_Sample_DockerDecoratorEnabledByDefault(t *testing.T) {
	ctx := new(mocks.AgentContext)
	cfg := config.NewConfig()
	ctx.On("Config").Times(4).Return(cfg)

	containerSamplerGetter = func(cacheTTL time.Duration, dockerAPIVersion, dockerContainerdNamespace string) []metrics.ContainerSampler {
		return []metrics.ContainerSampler{&fakeContainerSampler{}}
	}

	defer func() {
		containerSamplerGetter = metrics.GetContainerSamplers
	}()

	expected := []metrics.ContainerSampler{&fakeContainerSampler{}}
	sampler := NewProcessSampler(ctx).(*processSampler) //nolint:forcetypeassert
	assert.Equal(t, expected, sampler.containerSamplers)
}

//nolint:paralleltest
func TestProcessSampler_Sample_DockerDecoratorEnabledWithNoConfig(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Times(2).Return(nil)

	containerSamplerGetter = func(cacheTTL time.Duration, dockerAPIVersion, dockerContainerdNamespace string) []metrics.ContainerSampler {
		return []metrics.ContainerSampler{&fakeContainerSampler{}}
	}

	defer func() {
		containerSamplerGetter = metrics.GetContainerSamplers
	}()

	expected := []metrics.ContainerSampler{&fakeContainerSampler{}}
	sampler := NewProcessSampler(ctx).(*processSampler) //nolint:forcetypeassert
	assert.Equal(t, expected, sampler.containerSamplers)
}

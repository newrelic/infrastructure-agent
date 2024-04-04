// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build harvest
// +build harvest

package harvest

import (
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostid"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/stretchr/testify/assert"
)

func TestUptime(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	storageSampler := storage.NewSampler(ctx)

	hostIDProvider := &hostid.ProviderMock{}
	hostIDProvider.On("Provide").Return("some-host-id", nil)

	systemSampler := metrics.NewSystemSampler(ctx, storageSampler, nil, hostIDProvider)

	sampleB1, _ := systemSampler.Sample()
	sample1 := sampleB1[0].(*metrics.SystemSample)
	// wait 3 seconds
	var seconds uint64 = 3
	time.Sleep(time.Second * time.Duration(seconds))
	sampleB2, _ := systemSampler.Sample()
	sample2 := sampleB2[0].(*metrics.SystemSample)
	spentSeconds := sample2.Uptime - sample1.Uptime

	assert.LessOrEqual(t, spentSeconds, seconds+1)
	assert.GreaterOrEqual(t, spentSeconds, seconds)
}

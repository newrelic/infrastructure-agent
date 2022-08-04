// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	metricsSender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
)

func RegisterPlugins(a *agent.Agent) error {

        storageSampler := storage.NewSampler(a.Context)
        sender := metricsSender.NewSender(a.Context)
        systemSampler := metrics.NewSystemSampler(a.Context, storageSampler)

        sender.RegisterSampler(systemSampler)

        a.RegisterMetricsSender(sender)

        return nil
}

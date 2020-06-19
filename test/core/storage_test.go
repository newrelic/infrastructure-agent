// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package core

import (
	"testing"

	metrics_sender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/sample"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
)

func TestStorageSample(t *testing.T) {
	sample := fixture.StorageSample

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client)

	sender := metrics_sender.NewSender(a.Context)
	sender.RegisterSampler(fixture.NewSampler(&sample))
	a.RegisterMetricsSender(sender)

	go a.Run()

	req := <-testClient.RequestCh
	a.Terminate()

	fixture.AssertRequestContainsSample(t, req, &sample)
}

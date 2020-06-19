// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	metrics_sender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"

	"github.com/stretchr/testify/assert"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
)

func TestHeartBeatSampler(t *testing.T) {
	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.DisplayName = "my_display_name"
		config.IsSecureForwardOnly = true
		config.HeartBeatSampleRate = 1
	})

	sender := metrics_sender.NewSender(a.Context)
	heartBeatSampler := metrics.NewHeartbeatSampler(a.Context)
	sender.RegisterSampler(heartBeatSampler)
	a.RegisterMetricsSender(sender)
	go a.Run()

	var req http.Request
	select {
	case req = <-testClient.RequestCh:
		a.Terminate()
	case <-time.After(timeout):
		a.Terminate()
		assert.FailNow(t, "timeout while waiting for a response")
	}

	body, readErr := ioutil.ReadAll(req.Body)
	assert.NoError(t, readErr)

	assert.Contains(t, string(body), "heartBeatCounter")
}

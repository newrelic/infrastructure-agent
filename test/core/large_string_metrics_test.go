// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package core

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	metrics_sender "github.com/newrelic/infrastructure-agent/pkg/metrics/sender"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/sample"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/newrelic/infrastructure-agent/test/proxy/minagent"
)

func TestLimitLargeString_Struct(t *testing.T) {
	inputSample := fixture.ProcessSample
	inputSample.CmdLine = generateString(7000)

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.TruncTextValues = true
	})

	sender := metrics_sender.NewSender(a.Context)
	sender.RegisterSampler(fixture.NewSampler(&inputSample))
	a.RegisterMetricsSender(sender)

	go a.Run()

	req := <-testClient.RequestCh
	a.Terminate()

	expected := fixture.ProcessSample
	expected.Entity("display-name")
	expected.CmdLine = generateString(4095)

	fixture.AssertRequestContainsSample(t, req, &expected)
}

func TestLimitLargeString_Map(t *testing.T) {
	inputSample := minagent.FakeSample{
		"fake_foo": generateString(4095),
		"fake_bar": "hello",
	}

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(cfg *config.Config) {
		cfg.TruncTextValues = true
	})

	sender := metrics_sender.NewSender(a.Context)
	sender.RegisterSampler(fixture.NewSampler(&inputSample))
	a.RegisterMetricsSender(sender)

	go a.Run()

	req := <-testClient.RequestCh
	a.Terminate()

	expected := minagent.FakeSample{
		"fake_foo":  generateString(4095),
		"fake_bar":  "hello",
		"entityKey": "display-name",
	}

	fixture.AssertRequestContainsSample(t, req, expected)
}

func generateString(length int) string {
	encode := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+-")
	str := make([]byte, length)
	for i := 0; i < length; i++ {
		str[i] = encode[i%len(encode)]
	}
	return string(str)
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"github.com/newrelic/infrastructure-agent/internal/testhelpers"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertNetworkSample(t *testing.T, sample *network.NetworkSample) {
	assert.NotNil(t, sample.ReceiveBytesPerSec)
	assert.NotNil(t, sample.ReceivePacketsPerSec)
	assert.NotNil(t, sample.ReceiveErrorsPerSec)
	assert.NotNil(t, sample.ReceiveDroppedPerSec)
	assert.NotNil(t, sample.TransmitBytesPerSec)
	assert.NotNil(t, sample.TransmitPacketsPerSec)
	assert.NotNil(t, sample.TransmitErrorsPerSec)
	assert.NotNil(t, sample.TransmitDroppedPerSec)
}

func compareNetworkSamples(t *testing.T, sample1 *network.NetworkSample, sample2 *network.NetworkSample) {
	assert.True(t, *sample1.ReceiveBytesPerSec < *sample2.ReceiveBytesPerSec)
	assert.True(t, *sample1.ReceivePacketsPerSec < *sample2.ReceivePacketsPerSec)
	assert.True(t, *sample1.TransmitBytesPerSec < *sample2.TransmitBytesPerSec)
	assert.True(t, *sample1.TransmitPacketsPerSec < *sample2.TransmitPacketsPerSec)
}

func getLoNetworkSample(t *testing.T, networkSampler *network.NetworkSampler) *network.NetworkSample {
	var sample *network.NetworkSample
	foundLo := false
	samples, err := networkSampler.Sample()
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, len(samples) > 0)

	for _, sampleI := range samples {
		sample = sampleI.(*network.NetworkSample)
		if sample.InterfaceName == "lo" {
			assert.Equal(t, "127.0.0.1/8", sample.IpV4Address)
			assertNetworkSample(t, sample)
			foundLo = true
			break
		}
	}
	require.True(t, foundLo)
	return sample
}

func httpHandler(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	fmt.Fprint(res, "pong-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZu")
}

func TestNetwork(t *testing.T) {
	ctx := new(mocks.AgentContext)
	ctx.On("Config").Return(&config.Config{
		MetricsNetworkSampleRate: 1,
	})
	networkSampler := network.NewNetworkSampler(ctx)

	// We need an initial sample to set the lastRun values for calculating
	// the deltas
	_, err := networkSampler.Sample()
	if err != nil {
		t.Fatal(err)
	}

	sample1 := getLoNetworkSample(t, networkSampler)

	// Increase Received/Transmitted samples on the lo interface by creating
	// an http server binds to the interface and makes requests to it.
	http.HandleFunc("/", httpHandler)
	port, err := testhelpers.GetFreeTCPPort()
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(listener, nil)

	for i := 0; i < 100; i++ {
		r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if err != nil {
			t.Log(err)
		}
		assert.Equal(t, 200, r.StatusCode)
	}

	listener.Close()

	// Assert that the network read/write increase
	sample2 := getLoNetworkSample(t, networkSampler)
	compareNetworkSamples(t, sample1, sample2)

	// Assert that the network read/write decrease
	sample3 := getLoNetworkSample(t, networkSampler)
	compareNetworkSamples(t, sample3, sample2)
}

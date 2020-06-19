// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sample"

	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
)

var nslog = log.WithComponent("NetworkSampler")

const (
	STATE_DOWN    = "down"
	STATE_UP      = "up"
	FLAG_STATE_UP = "up"
)

// We use pointers to floats instead of plain floats so that if we don't set one
// of the values, it will not be sent to Dirac. (Not using pointers would mean
// that Go would always send a default value of 0.)
type NetworkSample struct {
	sample.BaseEvent

	InterfaceName   string `json:"interfaceName"`
	HardwareAddress string `json:"hardwareAddress"`
	IpV4Address     string `json:"ipV4Address,omitempty"`
	IpV6Address     string `json:"ipV6Address,omitempty"`
	State           string `json:"state,omitempty"`

	ReceiveBytesPerSec   *float64 `json:"receiveBytesPerSecond,omitempty"`
	ReceivePacketsPerSec *float64 `json:"receivePacketsPerSecond,omitempty"`
	ReceiveErrorsPerSec  *float64 `json:"receiveErrorsPerSecond,omitempty"`
	ReceiveDroppedPerSec *float64 `json:"receiveDroppedPerSecond,omitempty"`

	TransmitBytesPerSec   *float64 `json:"transmitBytesPerSecond,omitempty"`
	TransmitPacketsPerSec *float64 `json:"transmitPacketsPerSecond,omitempty"`
	TransmitErrorsPerSec  *float64 `json:"transmitErrorsPerSecond,omitempty"`
	TransmitDroppedPerSec *float64 `json:"transmitDroppedPerSecond,omitempty"`
}

func NewNetworkSampler(context agent.AgentContext) *NetworkSampler {
	samplerIntervalSec := config.FREQ_INTERVAL_FLOOR_NETWORK_METRICS
	if context != nil {
		samplerIntervalSec = context.Config().MetricsNetworkSampleRate
	}

	return &NetworkSampler{
		context:        context,
		waitForCleanup: &sync.WaitGroup{},
		sampleInterval: time.Second * time.Duration(samplerIntervalSec),
	}
}

func (ns *NetworkSampler) Debug() bool {
	if ns.context == nil {
		return false
	}
	return ns.context.Config().Debug
}

func (ns *NetworkSampler) Name() string { return "NetworkSampler" }

func (ns *NetworkSampler) Interval() time.Duration {
	return ns.sampleInterval
}

func (ns *NetworkSampler) Disabled() bool {
	return ns.Interval() <= config.FREQ_DISABLE_SAMPLING
}

func (ns *NetworkSampler) OnStartup() {}

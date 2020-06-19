// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import (
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var neworkTraffic = struct {
	ReceivedTraffic float64
	TransmitTraffic float64
	Errors          float64
	Packet          float64
	Dropped         float64
}{
	12121212.1212,
	6666666.66,
	2.5,
	40.2,
	10.5,
}

var NetworkSample = network.NetworkSample{
	BaseEvent: sample.BaseEvent{
		EntityKey: "my-entity-key",
	},
	InterfaceName:         "eth0",
	HardwareAddress:       "00:0a:95:9d:68:16",
	IpV4Address:           "172.168.1.2",
	IpV6Address:           "0:0:0:0:0:ffff:aca8:102",
	State:                 "up",
	ReceiveBytesPerSec:    &neworkTraffic.ReceivedTraffic,
	ReceivePacketsPerSec:  &neworkTraffic.Packet,
	ReceiveErrorsPerSec:   &neworkTraffic.Errors,
	ReceiveDroppedPerSec:  &neworkTraffic.Dropped,
	TransmitBytesPerSec:   &neworkTraffic.TransmitTraffic,
	TransmitPacketsPerSec: &neworkTraffic.Packet,
	TransmitErrorsPerSec:  &neworkTraffic.Errors,
	TransmitDroppedPerSec: &neworkTraffic.Dropped,
}

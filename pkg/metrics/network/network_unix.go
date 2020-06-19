// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package network

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/acquire"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/shirou/gopsutil/net"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	network_helpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
)

type NetworkSampler struct {
	context         agent.AgentContext
	lastRun         time.Time
	lastNetStats    map[string]net.IOCountersStat
	hasBootstrapped bool
	stopChannel     chan bool
	waitForCleanup  *sync.WaitGroup
	sampleInterval  time.Duration
}

// Returns false if the given network stats should not be added to the "All" total.
// func shouldAccumulateInterface(netStat net.IOCountersStat) bool {
// 	if strings.HasPrefix(netStat.Name, "bond") {
// 		return false
// 	}
// 	return true
// }

func (ss *NetworkSampler) Sample() (results sample.EventBatch, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in NetworkSampler.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	var cfg *config.Config
	if ss.context != nil {
		cfg = ss.context.Config()
	}

	var elapsedMs int64
	var elapsedSeconds float64
	now := time.Now()
	if ss.hasBootstrapped {
		elapsedMs = (now.UnixNano() - ss.lastRun.UnixNano()) / 1000000
	}
	elapsedSeconds = float64(elapsedMs) / 1000
	ss.lastRun = now
	ss.hasBootstrapped = true

	niList, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	if ss.Debug() {
		helpers.LogStructureDetails(nslog, niList, "NetInterfaces", "raw", nil)
	}

	var networkInterfaceFilters map[string][]string
	if cfg != nil {
		networkInterfaceFilters = cfg.NetworkInterfaceFilters
	}

	reportedInterfaces := make(map[string]*NetworkSample)
	for _, ni := range niList {

		if network_helpers.ShouldIgnoreInterface(networkInterfaceFilters, ni.Name) {
			continue
		}

		sample := &NetworkSample{}
		sample.Type("NetworkSample")

		sample.InterfaceName = ni.Name
		sample.HardwareAddress = ni.HardwareAddr

		sample.State = STATE_DOWN
		for _, flag := range ni.Flags {
			if flag == FLAG_STATE_UP {
				sample.State = STATE_UP
				break
			}
		}

		ipv4, ipv6 := network_helpers.IPAddressesByType(ni.Addrs)
		sample.IpV4Address = ipv4
		sample.IpV6Address = ipv6

		reportedInterfaces[ni.Name] = sample
		results = append(results, sample)
	}

	ioCounters, err := net.IOCounters(true)

	if err != nil {
		return nil, err
	}

	if ss.Debug() {
		helpers.LogStructureDetails(nslog, ioCounters, "IOCounters", "raw", nil)
	}

	nextNetStats := make(map[string]net.IOCountersStat)
	for _, counter := range ioCounters {
		if ss.lastNetStats != nil {
			interfaceName := counter.Name
			sample := reportedInterfaces[interfaceName]
			if sample == nil {
				continue
			}

			if lastStats, ok := ss.lastNetStats[interfaceName]; ok {
				bytesSent := acquire.CalculateSafeDelta(counter.BytesSent, lastStats.BytesSent, elapsedSeconds)
				bytesRecv := acquire.CalculateSafeDelta(counter.BytesRecv, lastStats.BytesRecv, elapsedSeconds)

				packetsSent := acquire.CalculateSafeDelta(counter.PacketsSent, lastStats.PacketsSent, elapsedSeconds)
				packetsRecv := acquire.CalculateSafeDelta(counter.PacketsRecv, lastStats.PacketsRecv, elapsedSeconds)

				errSent := acquire.CalculateSafeDelta(counter.Errout, lastStats.Errout, elapsedSeconds)
				errRecv := acquire.CalculateSafeDelta(counter.Errin, lastStats.Errin, elapsedSeconds)

				dropSent := acquire.CalculateSafeDelta(counter.Dropout, lastStats.Dropout, elapsedSeconds)
				dropRecv := acquire.CalculateSafeDelta(counter.Dropin, lastStats.Dropin, elapsedSeconds)

				sample.TransmitBytesPerSec = &bytesSent
				sample.TransmitPacketsPerSec = &packetsSent
				sample.TransmitErrorsPerSec = &errSent
				sample.TransmitDroppedPerSec = &dropSent

				sample.ReceiveBytesPerSec = &bytesRecv
				sample.ReceivePacketsPerSec = &packetsRecv
				sample.ReceiveErrorsPerSec = &errRecv
				sample.ReceiveDroppedPerSec = &dropRecv
			}
		}
		nextNetStats[counter.Name] = counter
	}
	ss.lastNetStats = nextNetStats

	if ss.Debug() {
		for _, sample := range results {
			helpers.LogStructureDetails(nslog, sample.(*NetworkSample), "NetworkSample", "final", nil)
		}
	}
	return results, nil
}

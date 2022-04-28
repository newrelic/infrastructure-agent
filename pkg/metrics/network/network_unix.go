// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux || darwin
// +build linux darwin

package network

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/acquire"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/shirou/gopsutil/v3/net"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	network_helpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
)

type NetworkSampler struct {
	context                 agent.AgentContext
	lastRun                 time.Time
	lastNetStats            map[string]net.IOCountersStat
	hasBootstrapped         bool
	stopChannel             chan bool
	waitForCleanup          *sync.WaitGroup
	sampleInterval          time.Duration
	networkInterfaceFilters map[string][]string
	debug                   bool
}

// Returns false if the given network stats should not be added to the "All" total.
// func shouldAccumulateInterface(netStat net.IOCountersStat) bool {
// 	if strings.HasPrefix(netStat.Name, "bond") {
// 		return false
// 	}
// 	return true
// }

func (ns *NetworkSampler) Sample() (results sample.EventBatch, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in NetworkSampler.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	var elapsedMs int64
	var elapsedSeconds float64
	now := time.Now()
	if ns.hasBootstrapped {
		elapsedMs = (now.UnixNano() - ns.lastRun.UnixNano()) / 1000000
	}
	elapsedSeconds = float64(elapsedMs) / 1000
	ns.lastRun = now
	ns.hasBootstrapped = true

	niList, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	if ns.debug {
		helpers.LogStructureDetails(nslog, niList, "NetInterfaces", "raw", nil)
	}

	reportedInterfaces := make(map[string]*NetworkSample)
	for _, ni := range niList {

		if network_helpers.ShouldIgnoreInterface(ns.networkInterfaceFilters, ni.Name) {
			continue
		}

		s := &NetworkSample{}
		s.Type("NetworkSample")

		s.InterfaceName = ni.Name
		s.HardwareAddress = ni.HardwareAddr

		s.State = STATE_DOWN
		for _, flag := range ni.Flags {
			if flag == FLAG_STATE_UP {
				s.State = STATE_UP
				break
			}
		}

		ipv4, ipv6 := network_helpers.IPAddressesByType(ni.Addrs)
		s.IpV4Address = ipv4
		s.IpV6Address = ipv6

		reportedInterfaces[ni.Name] = s
		results = append(results, s)
	}

	ioCounters, err := net.IOCounters(true)

	if err != nil {
		return nil, err
	}

	if ns.debug {
		helpers.LogStructureDetails(nslog, ioCounters, "IOCounters", "raw", nil)
	}

	nextNetStats := make(map[string]net.IOCountersStat)
	for _, counter := range ioCounters {
		if ns.lastNetStats != nil {
			interfaceName := counter.Name
			s := reportedInterfaces[interfaceName]
			if s == nil {
				continue
			}

			if lastStats, ok := ns.lastNetStats[interfaceName]; ok {
				bytesSent := acquire.CalculateSafeDelta(counter.BytesSent, lastStats.BytesSent, elapsedSeconds)
				bytesRecv := acquire.CalculateSafeDelta(counter.BytesRecv, lastStats.BytesRecv, elapsedSeconds)

				packetsSent := acquire.CalculateSafeDelta(counter.PacketsSent, lastStats.PacketsSent, elapsedSeconds)
				packetsRecv := acquire.CalculateSafeDelta(counter.PacketsRecv, lastStats.PacketsRecv, elapsedSeconds)

				errSent := acquire.CalculateSafeDelta(counter.Errout, lastStats.Errout, elapsedSeconds)
				errRecv := acquire.CalculateSafeDelta(counter.Errin, lastStats.Errin, elapsedSeconds)

				dropSent := acquire.CalculateSafeDelta(counter.Dropout, lastStats.Dropout, elapsedSeconds)
				dropRecv := acquire.CalculateSafeDelta(counter.Dropin, lastStats.Dropin, elapsedSeconds)

				s.TransmitBytesPerSec = &bytesSent
				s.TransmitPacketsPerSec = &packetsSent
				s.TransmitErrorsPerSec = &errSent
				s.TransmitDroppedPerSec = &dropSent

				s.ReceiveBytesPerSec = &bytesRecv
				s.ReceivePacketsPerSec = &packetsRecv
				s.ReceiveErrorsPerSec = &errRecv
				s.ReceiveDroppedPerSec = &dropRecv
			}
		}
		nextNetStats[counter.Name] = counter
	}
	ns.lastNetStats = nextNetStats

	if ns.debug {
		for _, s := range results {
			helpers.LogStructureDetails(nslog, s.(*NetworkSample), "NetworkSample", "final", nil)
		}
	}
	return results, nil
}

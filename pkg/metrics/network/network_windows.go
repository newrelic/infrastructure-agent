// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package network

import (
	"fmt"

	network_helpers "github.com/newrelic/infrastructure-agent/pkg/helpers/network"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/acquire"
	"github.com/sirupsen/logrus"

	"net"
	"os"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/sample"
	gopsnet "github.com/shirou/gopsutil/net"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

type NetworkSampler struct {
	context         agent.AgentContext
	lastRun         time.Time
	lastNetStats    map[uint32]IOCountersWithIndexStat
	hasBootstrapped bool
	stopChannel     chan bool
	waitForCleanup  *sync.WaitGroup
	sampleInterval  time.Duration
}

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

	niList, err := InterfacesWithIndex()
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

	reportedInterfaces := make(map[uint32]*NetworkSample)
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

		reportedInterfaces[ni.Index] = sample
		results = append(results, sample)
	}

	ioCounters, err := IOCountersForInterface(niList)
	if err != nil {
		return nil, err
	}

	if ss.Debug() {
		helpers.LogStructureDetails(nslog, ioCounters, "IOCounters", "raw", nil)
	}

	nextNetStats := make(map[uint32]IOCountersWithIndexStat)
	for _, counter := range ioCounters {
		if ss.lastNetStats != nil {
			sample := reportedInterfaces[counter.Index]
			if sample == nil {
				continue
			}

			if lastStats, ok := ss.lastNetStats[counter.Index]; ok {
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
		nextNetStats[counter.Index] = counter
	}
	ss.lastNetStats = nextNetStats

	if ss.Debug() {
		for _, sample := range results {
			helpers.LogStructureDetails(nslog, sample.(*NetworkSample), "NetworkSample", "final", nil)
		}
	}
	return results, nil
}

type IOCountersWithIndexStat struct {
	Name        string `json:"name"`        // interface name
	BytesSent   uint64 `json:"bytesSent"`   // number of bytes sent
	BytesRecv   uint64 `json:"bytesRecv"`   // number of bytes received
	PacketsSent uint64 `json:"packetsSent"` // number of packets sent
	PacketsRecv uint64 `json:"packetsRecv"` // number of packets received
	Errin       uint64 `json:"errin"`       // total number of errors while receiving
	Errout      uint64 `json:"errout"`      // total number of errors while sending
	Dropin      uint64 `json:"dropin"`      // total number of incoming packets which were dropped
	Dropout     uint64 `json:"dropout"`     // total number of outgoing packets which were dropped (always 0 on OSX and BSD)
	Fifoin      uint64 `json:"fifoin"`      // total number of FIFO buffers errors while receiving
	Fifoout     uint64 `json:"fifoout"`     // total number of FIFO buffers errors while sending
	Index       uint32 `json:"index"`
}

type InterfaceWithIndexStat struct {
	MTU          int                     `json:"mtu"`          // maximum transmission unit
	Name         string                  `json:"name"`         // e.g., "en0", "lo0", "eth0.100"
	HardwareAddr string                  `json:"hardwareaddr"` // IEEE MAC-48, EUI-48 and EUI-64 form
	Flags        []string                `json:"flags"`        // e.g., FlagUp, FlagLoopback, FlagMulticast
	Addrs        []gopsnet.InterfaceAddr `json:"addrs"`
	Index        uint32                  `json:"index"`
}

func InterfacesWithIndex() ([]InterfaceWithIndexStat, error) {
	is, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ret := make([]InterfaceWithIndexStat, 0, len(is))
	for _, ifi := range is {

		var flags []string
		if ifi.Flags&net.FlagUp != 0 {
			flags = append(flags, "up")
		}
		if ifi.Flags&net.FlagBroadcast != 0 {
			flags = append(flags, "broadcast")
		}
		if ifi.Flags&net.FlagLoopback != 0 {
			flags = append(flags, "loopback")
		}
		if ifi.Flags&net.FlagPointToPoint != 0 {
			flags = append(flags, "pointtopoint")
		}
		if ifi.Flags&net.FlagMulticast != 0 {
			flags = append(flags, "multicast")
		}

		r := InterfaceWithIndexStat{
			Index:        uint32(ifi.Index),
			Name:         ifi.Name,
			MTU:          ifi.MTU,
			HardwareAddr: ifi.HardwareAddr.String(),
			Flags:        flags,
		}

		nslog.WithFieldsF(func() logrus.Fields {
			return logrus.Fields{
				"index": r.Index,
				"name":  r.Name,
			}
		}).Debug("INTERFACE resolved.")

		addrs, err := ifi.Addrs()
		if err == nil {
			r.Addrs = make([]gopsnet.InterfaceAddr, 0, len(addrs))
			for _, addr := range addrs {
				r.Addrs = append(r.Addrs, gopsnet.InterfaceAddr{
					Addr: addr.String(),
				})
			}

		}
		ret = append(ret, r)
	}

	return ret, nil
}

func IOCountersForInterface(ifs []InterfaceWithIndexStat) ([]IOCountersWithIndexStat, error) {

	var ret []IOCountersWithIndexStat

	for _, ifi := range ifs {
		nslog.WithFieldsF(func() logrus.Fields {
			return logrus.Fields{
				"index": ifi.Index,
				"name":  ifi.Name,
			}
		}).Debug("IOCOUNTER resolved.")

		c := IOCountersWithIndexStat{
			Name:  ifi.Name,
			Index: ifi.Index,
		}

		row := syscall.MibIfRow{Index: ifi.Index}
		e := syscall.GetIfEntry(&row)
		if e != nil {
			return nil, os.NewSyscallError("GetIfEntry", e)
		}
		c.BytesSent = uint64(row.OutOctets)
		c.BytesRecv = uint64(row.InOctets)
		c.PacketsSent = uint64(row.OutUcastPkts)
		c.PacketsRecv = uint64(row.InUcastPkts)
		c.Errin = uint64(row.InErrors)
		c.Errout = uint64(row.OutErrors)
		c.Dropin = uint64(row.InDiscards)
		c.Dropout = uint64(row.OutDiscards)

		ret = append(ret, c)
	}

	return ret, nil
}

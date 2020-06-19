// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux

package nfs

import (
	"fmt"
	"math"
	"time"

	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/disk"
)

func populateNFS(cache map[string]statsCache, detailed bool) ([]*Sample, error) {
	mounts, err := getMounts()
	if err != nil {
		return nil, fmt.Errorf("error retrieving mounts for NFS: %s", err)
	}

	checkTime := time.Now()
	samples := []*Sample{}
	for _, m := range mounts {
		if m.Type == "nfs" || m.Type == "nfs4" {
			sample, err := parseNFSMount(cache, m, checkTime)
			if err != nil {
				return nil, err
			}
			if detailed {
				parseDetailedNFSStats(sample, m.Stats.(*procfs.MountStatsNFS))
			}
			samples = append(samples, sample)
		}
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no supported NFS mounts found")
	}

	return samples, nil
}

func getMounts() ([]*procfs.Mount, error) {
	proc, err := procfs.Self()
	if err != nil {
		return nil, err
	}
	return proc.MountStats()
}

func parseNFSMount(cache map[string]statsCache, mount *procfs.Mount, checkTime time.Time) (*Sample, error) {
	ms := mount.Stats.(*procfs.MountStatsNFS)

	diskStats, err := disk.Usage(mount.Mount)
	if err != nil {
		return nil, err
	}
	diskFreePercent := (float64(diskStats.Free) / float64(diskStats.Total)) * 100
	s := &Sample{
		DiskTotalBytes:  &diskStats.Total,
		DiskUsedBytes:   &diskStats.Used,
		DiskUsedPercent: parseFloat(diskStats.UsedPercent),
		DiskFreeBytes:   &diskStats.Free,
		DiskFreePercent: parseFloat(diskFreePercent),
		TotalReadBytes:  &ms.Bytes.ReadTotal,
		TotalWriteBytes: &ms.Bytes.WriteTotal,
		Device:          &mount.Device,
		Mountpoint:      &mount.Mount,
		FilesystemType:  &mount.Type,
	}
	if v, ok := ms.Opts["vers"]; ok {
		s.Version = &v
	}

	if lms, ok := cache[mount.Mount]; ok {
		populateNFSPerSecMetrics(lms, ms.Operations, s, checkTime)
	}
	cache[mount.Mount] = statsCache{
		last:    ms,
		lastRun: checkTime,
	}
	return s, nil
}

func parseDetailedNFSStats(sample *Sample, ms *procfs.MountStatsNFS) {
	sample.Age = parseFloat(ms.Age.Seconds())
	sample.InodeRevalidate = &ms.Events.InodeRevalidate
	sample.DnodeRevalidate = &ms.Events.DnodeRevalidate
	sample.DataInvalidate = &ms.Events.DataInvalidate
	sample.AttributeInvalidate = &ms.Events.AttributeInvalidate
	sample.VFSOpen = &ms.Events.VFSOpen
	sample.VFSLookup = &ms.Events.VFSLookup
	sample.VFSAccess = &ms.Events.VFSAccess
	sample.VFSUpdatePage = &ms.Events.VFSUpdatePage
	sample.VFSReadPage = &ms.Events.VFSReadPage
	sample.VFSReadPages = &ms.Events.VFSReadPages
	sample.VFSWritePage = &ms.Events.VFSWritePage
	sample.VFSWritePages = &ms.Events.VFSWritePages
	sample.VFSGetdents = &ms.Events.VFSGetdents
	sample.VFSSetattr = &ms.Events.VFSSetattr
	sample.VFSFlush = &ms.Events.VFSFlush
	sample.VFSFsync = &ms.Events.VFSFsync
	sample.VFSLock = &ms.Events.VFSLock
	sample.VFSFileRelease = &ms.Events.VFSFileRelease
	sample.Truncation = &ms.Events.Truncation
	sample.WriteExtension = &ms.Events.WriteExtension
	sample.SillyRename = &ms.Events.SillyRename
	sample.ShortRead = &ms.Events.ShortRead
	sample.ShortWrite = &ms.Events.ShortWrite
	sample.JukeboxDelay = &ms.Events.JukeboxDelay
	sample.PNFSRead = &ms.Events.PNFSRead
	sample.PNFSWrite = &ms.Events.PNFSWrite
	sample.Bind = &ms.Transport.Bind
	sample.Connect = &ms.Transport.Connect
	sample.ConnectIdleTime = &ms.Transport.ConnectIdleTime
	sample.IdleTimeSeconds = &ms.Transport.IdleTimeSeconds
	sample.Sends = &ms.Transport.Sends
	sample.Receives = &ms.Transport.Receives
	sample.BadTransactionIDs = &ms.Transport.BadTransactionIDs
	sample.CumulativeActiveRequests = &ms.Transport.CumulativeActiveRequests
	sample.CumulativeBacklog = &ms.Transport.CumulativeBacklog
	sample.MaximumRPCSlotsUsed = &ms.Transport.MaximumRPCSlotsUsed
	sample.CumulativeSendingQueue = &ms.Transport.CumulativeSendingQueue
	sample.CumulativePendingQueue = &ms.Transport.CumulativePendingQueue
}

func populateNFSPerSecMetrics(lms statsCache, ops []procfs.NFSOperationStats, sample *Sample, checkTime time.Time) {
	lastRun := lms.lastRun
	total, reads, writes := compareNFSOps(lms.last.Operations, ops, lastRun, checkTime)
	sample.TotalOpsPerSec = &total
	sample.ReadsPerSec = &reads
	sample.WritesPerSec = &writes

	readBytesPerSec := nfsStatDelta(lms.last.Bytes.ReadTotal, *sample.TotalReadBytes, lastRun, checkTime)
	writeBytesPerSec := nfsStatDelta(lms.last.Bytes.WriteTotal, *sample.TotalWriteBytes, lastRun, checkTime)
	sample.ReadBytesPerSec = &readBytesPerSec
	sample.WriteBytesPerSec = &writeBytesPerSec
}

func compareNFSOps(last, current []procfs.NFSOperationStats, lastRun, checkTime time.Time) (float64, float64, float64) {
	lastTotal, lastRead, lastWrite := parseNFSOps(last)
	currentTotal, currentRead, currentWrite := parseNFSOps(current)

	total := nfsStatDelta(lastTotal, currentTotal, lastRun, checkTime)
	read := nfsStatDelta(lastRead, currentRead, lastRun, checkTime)
	write := nfsStatDelta(lastWrite, currentWrite, lastRun, checkTime)

	return total, read, write
}

func parseNFSOps(ops []procfs.NFSOperationStats) (total uint64, read uint64, write uint64) {
	for _, op := range ops {
		total += op.Requests
		switch op.Operation {
		case "READ":
			read += op.Requests
		case "WRITE":
			write += op.Requests
		}
	}
	return total, read, write
}

func nfsStatDelta(last, current uint64, lastRun, checkTime time.Time) float64 {
	timeDelta := checkTime.Sub(lastRun).Seconds()
	if delta := float64(current-last) / timeDelta; !nanOrInf(delta) {
		return delta
	}
	return 0
}

func parseFloat(f float64) *float64 {
	if nanOrInf(f) {
		return nil
	}
	return &f
}

func nanOrInf(f float64) bool {
	if math.IsNaN(f) || math.IsInf(f, 0) || math.IsInf(f, 1) {
		return true
	}
	return false
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package metrics

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"regexp"
	"runtime/debug"
)

var diskRegex = regexp.MustCompile("^/dev/(disk[0-9]+)s[0-9]+(s[0-9]+)?$")

type storageData struct {
	totalUsedBytes  float64
	totalFreeBytes  float64
	totalBytes      float64
	diskUsedPercent float64
	diskFreePercent float64
}

type ioCountersData struct {
	readsPerSec             float64
	writesPerSec            float64
	percentUtilized         float64
	readUtilizationPercent  float64
	writeUtilizationPercent float64
}

func (m *DiskMonitor) Sample() (result *DiskSample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in DiskMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	if m.storageSampler == nil {
		return nil, fmt.Errorf("DiskMonitor is not properly configured with a storage sampler")
	}

	// make sure we don't count the sample device more than once
	samples := FilterStorageSamples(m.storageSampler.Samples())
	if len(samples) == 0 {
		return &DiskSample{}, nil
	}

	//All samples share same ElapsedSampleDeltaMs value
	elapsedMs := samples[0].ElapsedSampleDeltaMs

	sd := getStorageData(samples)
	ud := m.getUtilizationData(elapsedMs)

	result = &DiskSample{
		UsedBytes:               sd.totalUsedBytes,
		UsedPercent:             sd.diskUsedPercent,
		FreeBytes:               sd.totalFreeBytes,
		FreePercent:             sd.diskFreePercent,
		TotalBytes:              sd.totalBytes,
		UtilizationPercent:      ud.percentUtilized,
		ReadUtilizationPercent:  ud.readUtilizationPercent,
		WriteUtilizationPercent: ud.writeUtilizationPercent,
		ReadsPerSec:             ud.readsPerSec,
		WritesPerSec:            ud.writesPerSec,
	}

	return
}

// getUtilizationData return I/O related data based on gopsutil output
// gopsutil io_counters metrics are based on psutil
// https://psutil.readthedocs.io/en/latest/#psutil.disk_io_counters
func (m *DiskMonitor) getUtilizationData(elapsedMs int64) (cd ioCountersData) {
	lastDiskStats := m.storageSampler.LastDiskStats()
	ioCountersStats, err := m.storageSampler.SampleWrapper().IOCounters()
	if err != nil {
		syslog.WithError(err).Warn("cannot read ioCounters")
		return
	}

	return getUtilizationDataFromIoCountersDelta(elapsedMs, ioCountersStats, lastDiskStats)
}

// getUtilizationDataFromIoCountersDelta return I/O related delta data based on gopsutil output.
// Having separated functions for getting the data and processing it helps on testing it
func getUtilizationDataFromIoCountersDelta(elapsedMs int64, ioCountersStats, lastDiskStats map[string]storage.IOCountersStat) (cd ioCountersData) {
	if elapsedMs <= 0 {
		return
	}
	if len(lastDiskStats) == 0 || len(ioCountersStats) == 0 {
		return
	}

	var totalIOTime uint64
	var totalReadTime uint64
	var totalWriteTime uint64
	var totalReads uint64
	var totalWrites uint64

	var darwinCounterStat storage.DarwinIoCountersStat
	var darwinCounterLastStat storage.DarwinIoCountersStat
	for diskName := range ioCountersStats {
		darwinCounterStat = ioCountersStats[diskName].(storage.DarwinIoCountersStat)
		darwinCounterLastStat = lastDiskStats[diskName].(storage.DarwinIoCountersStat)

		totalReads += darwinCounterStat.ReadCount - darwinCounterLastStat.ReadCount
		totalWrites += darwinCounterStat.WriteCount - darwinCounterLastStat.WriteCount
		totalIOTime += darwinCounterStat.IoTime - darwinCounterLastStat.IoTime
		totalReadTime += darwinCounterStat.ReadTime - darwinCounterLastStat.ReadTime
		totalWriteTime += darwinCounterStat.WriteTime - darwinCounterLastStat.WriteTime
	}

	elapsedSeconds := float64(elapsedMs) / 1000
	cd.readsPerSec = float64(totalReads) / elapsedSeconds
	cd.writesPerSec = float64(totalWrites) / elapsedSeconds

	// Calculate rough utilization across whole machine
	var readPortion float64
	var writePortion float64

	numDevicesForIO := len(ioCountersStats)
	if numDevicesForIO > 0 {
		cd.percentUtilized = float64(totalIOTime) / float64(int64(numDevicesForIO)*elapsedMs) * 100
		if cd.percentUtilized > 100 {
			cd.percentUtilized = 100
		}
		readWriteTimeDelta := totalReadTime + totalWriteTime

		// Estimate which portion of the IO time was spent reading or writing
		// Basically, we break down how much time was spent reading and writing
		// (total R/W time, different than IO wait time).
		// If the disk spent 25% of the combined R/W time reading, that means we can
		// guess that read activity accounted for 25% of the total utilization percentage.
		if readWriteTimeDelta > 0 {
			readPortion = float64(totalReadTime) / float64(readWriteTimeDelta)
			writePortion = float64(totalWriteTime) / float64(readWriteTimeDelta)
		}
	}

	cd.readUtilizationPercent = cd.percentUtilized * readPortion
	cd.writeUtilizationPercent = cd.percentUtilized * writePortion

	return
}

// getStorageData returns all drives space related aggregated data
func getStorageData(samples []*storage.Sample) (sd storageData) {
	apfsPartitionsPerDisk := make(map[string]struct{})
	for _, ss := range samples {
		if !hasStorageData(ss) {
			continue
		}

		disk, errD := diskFromDevice(ss.Device)
		if errD != nil {
			syslog.WithError(errD).Warn("cannot obtain disk from device")
			continue
		}

		// APFS filesystems partitions share total size and free space so we only take into account
		// one APFS partition per disk for total bytes and free bytes
		if _, diskProcessed := apfsPartitionsPerDisk[disk]; ss.FileSystemType != "apfs" || !diskProcessed {
			sd.totalBytes += *ss.TotalBytes
			sd.totalFreeBytes += *ss.FreeBytes
			apfsPartitionsPerDisk[disk] = struct{}{}
		}
		sd.totalUsedBytes += *ss.UsedBytes
	}

	// overall used/free percentage for machine
	if sd.totalBytes > 0 {
		sd.diskUsedPercent = (sd.totalUsedBytes / sd.totalBytes) * 100
		sd.diskFreePercent = (sd.totalFreeBytes / sd.totalBytes) * 100
	}

	return
}

func hasStorageData(ss *storage.Sample) bool {
	if ss == nil {
		return false
	}
	if ss.TotalBytes == nil || ss.FreeBytes == nil || ss.UsedBytes == nil {
		return false
	}
	return true
}

// diskFromDevice returns the disk name from the device file in fs
// i.e. : /dev/disk1s1s1 --> disk1
func diskFromDevice(device string) (string, error) {
	matches := diskRegex.FindStringSubmatch(device)
	if len(matches) != 3 {
		return "", fmt.Errorf("cannot match disk from device %s", device)
	}
	return matches[1], nil
}

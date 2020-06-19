// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package metrics

import (
	"fmt"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

func (self *DiskMonitor) Sample() (result *DiskSample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in DiskMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	if self.storageSampler == nil {
		return nil, fmt.Errorf("DiskMonitor is not properly configured with a storage sampler")
	}

	// make sure we don't count the sample device more than once
	samples := FilterStorageSamples(self.storageSampler.Samples())

	var totalUsedBytes float64
	var totalFreeBytes float64
	var totalBytes float64
	var totalIOTime uint64
	var totalReadTime uint64
	var totalWriteTime uint64
	var totalReads uint64
	var totalWrites uint64
	var readsPerSec float64
	var writesPerSec float64
	var elapsedSeconds float64
	var elapsedMs int64

	for _, ss := range samples {

		totalBytes += *ss.TotalBytes
		totalUsedBytes += *ss.UsedBytes
		totalFreeBytes += *ss.FreeBytes

		// any delta value can be nil on first pass
		if ss.HasDelta {
			totalIOTime += ss.IOTimeDelta
			totalReadTime += ss.ReadTimeDelta
			totalWriteTime += ss.WriteTimeDelta
			totalReads += ss.ReadCountDelta
			totalWrites += ss.WriteCountDelta
		}

		// time fields _should_ all be the same for a give batch of
		// samples, so just take the last one
		elapsedMs = ss.ElapsedSampleDeltaMs
	}
	syslog.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"elapsedMs":      elapsedMs,
			"totalReadTime":  totalReadTime,
			"totalWriteTime": totalWriteTime,
			"totalReads":     totalReads,
			"totalWrites":    totalWrites,
		}
	}).Debug("Delta storage.")

	elapsedSeconds = float64(elapsedMs) / 1000
	if elapsedSeconds > 0 {
		readsPerSec = float64(totalReads) / elapsedSeconds
		writesPerSec = float64(totalWrites) / elapsedSeconds
	}

	// Calculate rough utilization across whole machine
	var percentUtilized float64
	var readPortion float64
	var writePortion float64
	numDevicesForIO := len(samples)
	if numDevicesForIO > 0 && elapsedMs > 0 {
		percentUtilized = float64(totalIOTime) / float64(int64(numDevicesForIO)*elapsedMs) * 100
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

	// overall used/free percentage for machine
	var diskUsedPercent float64
	var diskFreePercent float64
	if totalBytes > 0 {
		diskUsedPercent = (totalUsedBytes / totalBytes) * 100
		diskFreePercent = (totalFreeBytes / totalBytes) * 100
	}

	result = &DiskSample{
		UsedBytes:               totalUsedBytes,
		UsedPercent:             diskUsedPercent,
		FreeBytes:               totalFreeBytes,
		FreePercent:             diskFreePercent,
		TotalBytes:              totalBytes,
		UtilizationPercent:      percentUtilized,
		ReadUtilizationPercent:  percentUtilized * readPortion,
		WriteUtilizationPercent: percentUtilized * writePortion,
		ReadsPerSec:             readsPerSec,
		WritesPerSec:            writesPerSec,
	}
	return
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package metrics

import (
	"fmt"
	"runtime/debug"
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
	var readsPerSec float64 = 0
	var writesPerSec float64 = 0
	var totalTotalUtilizationPercent float64
	var totalReadUtilizationPercent float64
	var totalWriteUtilizationPercent float64

	for _, ss := range samples {
		// Should never be nil, but check
		if ss != nil {
			totalBytes += *ss.TotalBytes
			totalUsedBytes += *ss.UsedBytes
			totalFreeBytes += *ss.FreeBytes

			// These are pointers to floats, so can nil out if not
			// initialized. Now that we are pre-warming the sampler they
			// should always be populated.
			if ss.TotalUtilizationPercent != nil &&
				ss.ReadUtilizationPercent != nil &&
				ss.WriteUtilizationPercent != nil {
				totalTotalUtilizationPercent += *ss.TotalUtilizationPercent
				totalReadUtilizationPercent += *ss.ReadUtilizationPercent
				totalWriteUtilizationPercent += *ss.WriteUtilizationPercent
			} else {
				syslog.Debug("Storage sample utilization is empty.")
			}

			if ss.ReadsPerSec != nil {
				readsPerSec += *ss.ReadsPerSec
			}
			if ss.WritesPerSec != nil {
				writesPerSec += *ss.WritesPerSec
			}
		}
	}

	// Calculate rough utilization across whole machine
	numDevicesForIO := len(samples)
	if numDevicesForIO > 0 {
		// Utilization is currently defined to be an average of the %
		totalTotalUtilizationPercent /= float64(numDevicesForIO)
		totalReadUtilizationPercent /= float64(numDevicesForIO)
		totalWriteUtilizationPercent /= float64(numDevicesForIO)
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
		UtilizationPercent:      totalTotalUtilizationPercent,
		ReadUtilizationPercent:  totalReadUtilizationPercent,
		WriteUtilizationPercent: totalWriteUtilizationPercent,
		ReadsPerSec:             readsPerSec,
		WritesPerSec:            writesPerSec,
	}
	return
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/shirou/gopsutil/disk"
)

type Sample struct {
	BaseSample
}

type NullStorageSampleWrapper struct{}

func NewStorageSampleWrapper(cfg *config.Config) SampleWrapper {
	return &NullStorageSampleWrapper{}
}

func (ssw *NullStorageSampleWrapper) Partitions() (s []PartitionStat, e error) {
	return
}

func (ssw *NullStorageSampleWrapper) Usage(path string) (d *disk.UsageStat, e error) {
	return disk.Usage(path)
}

func (ssw *NullStorageSampleWrapper) IOCounters() (s map[string]IOCountersStat, e error) {
	return
}

func (ssw *NullStorageSampleWrapper) CalculateSampleValues(counter, lastStats IOCountersStat, elapsedMs int64) (s *Sample) {
	return
}

func populateSampleOS(source, dest *Sample) {
}

func populateUsageOS(_ *disk.UsageStat, _ *Sample) {
}

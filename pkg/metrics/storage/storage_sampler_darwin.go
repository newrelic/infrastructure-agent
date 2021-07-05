// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/shirou/gopsutil/disk"
	"regexp"
)

var (
	SupportedFileSystems = map[string]bool{}
	// TODO define types of partitions, volumes ...
	supportedDeviceRegexp = regexp.MustCompile("^/dev/([a-z0-9]+)")
)

type Sample struct {
	BaseSample
}

type DarwinStorageSampleWrapper struct{}

type DarwinIoCountersStat struct {
	disk.IOCountersStat
}

func (d DarwinIoCountersStat) Source() string {
	return "gopsutil"
}

func NewStorageSampleWrapper(cfg *config.Config) SampleWrapper {
	return &DarwinStorageSampleWrapper{}
}

func (ssw *DarwinStorageSampleWrapper) Partitions() (partitions []PartitionStat, e error) {
	partitionsInfo, err := disk.Partitions(true)
	if err != nil {
		return partitions, err
	}
	for _, p := range partitionsInfo {
		if !isSupportedDevice(p.Device) {
			continue
		}
		partitions = append(partitions, PartitionStat{
			Device:     p.Device,
			Mountpoint: p.Mountpoint,
			Fstype:     p.Fstype,
			Opts:       p.Opts,
		})
	}

	return partitions, nil
}

func isSupportedDevice(device string) bool {
	return supportedDeviceRegexp.Match([]byte(device))
}

func (ssw *DarwinStorageSampleWrapper) Usage(path string) (d *disk.UsageStat, e error) {
	return disk.Usage(path)
}

func (ssw *DarwinStorageSampleWrapper) IOCounters() (ioCounters map[string]IOCountersStat, e error) {
	ioCountersStat, err := disk.IOCounters()

	if err != nil {
		return ioCounters, err
	}

	ioCounters = make(map[string]IOCountersStat)
	for _, p := range ioCountersStat {
		ioCounters[p.Name] = DarwinIoCountersStat{p}
	}

	return ioCounters, nil
}

func (ssw *DarwinStorageSampleWrapper) CalculateSampleValues(counter, lastStats IOCountersStat, elapsedMs int64) (s *Sample) {
	return CalculateSampleValues(counter, lastStats, elapsedMs)
}

func populateSampleOS(source, dest *Sample) {
}

func populateUsageOS(_ *disk.UsageStat, _ *Sample) {
}

func CalculateDeviceMapping(activeDevices map[string]bool, _ bool) map[string]string {
	return nil
}

func CalculateSampleValues(ioCounter IOCountersStat, ioLastStats IOCountersStat, elapsedMs int64) *Sample {
	return nil
}

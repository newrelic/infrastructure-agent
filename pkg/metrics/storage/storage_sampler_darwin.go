// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/shirou/gopsutil/v3/disk"
	"strings"
	"sync"
	"time"
)

var (
	SupportedFileSystems = map[string]bool{
		"apfs":  true,
		"hfs":   true,
		"exfat": true,
	}
)

type Sample struct {
	BaseSample
}

type DarwinStorageSampleWrapper struct {
	partitionsCache PartitionsCache
	//concurrent access to disk using disk.IOCounters() with GCO enabled is no thread safe
	ioCountersLock sync.Mutex
}

type DarwinIoCountersStat struct {
	disk.IOCountersStat
}

func (d DarwinIoCountersStat) Source() string {
	return "gopsutil"
}

func NewStorageSampleWrapper(cfg *config.Config) SampleWrapper {
	ttl, err := time.ParseDuration(cfg.PartitionsTTL)
	if err != nil {
		ttl = time.Minute // for tests with an unset ttl
	}
	ssw := DarwinStorageSampleWrapper{
		partitionsCache: PartitionsCache{
			ttl:             ttl,
			isContainerized: cfg != nil && cfg.IsContainerized,
			partitionsFunc:  fetchPartitions,
		},
	}
	return &ssw
}

func (ssw *DarwinStorageSampleWrapper) Partitions() (partitions []PartitionStat, e error) {
	return ssw.partitionsCache.Get()
}

// fetchPartitions gets partitions information from gopsutil library
func fetchPartitions(_ bool) (partitions []PartitionStat, e error) {
	partitionsInfo, err := disk.Partitions(true)
	if err != nil {
		return partitions, err
	}

	return partitionsFromGopsutilPartitions(partitionsInfo), nil
}

func partitionsFromGopsutilPartitions(partitionsInfo []disk.PartitionStat) (partitions []PartitionStat) {
	for _, p := range partitionsInfo {
		if !isSupportedFs(p.Fstype) {
			continue
		}
		partitions = append(partitions, PartitionStat{
			Device:     p.Device,
			Mountpoint: p.Mountpoint,
			Fstype:     p.Fstype,
			Opts:       strings.Join(p.Opts, ","),
		})
	}

	return partitions
}

func isSupportedFs(fsType string) bool {
	_, supported := SupportedFileSystems[fsType]
	return supported
}

func (ssw *DarwinStorageSampleWrapper) Usage(path string) (d *disk.UsageStat, e error) {
	return disk.Usage(path)
}

func (ssw *DarwinStorageSampleWrapper) IOCounters() (ioCounters map[string]IOCountersStat, e error) {
	ssw.ioCountersLock.Lock()
	defer ssw.ioCountersLock.Unlock()

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

func (ssw *DarwinStorageSampleWrapper) CalculateSampleValues(_, _ IOCountersStat, _ int64) (s *Sample) {
	//IO per partition not supported yet in darwin
	return nil
}

// populateSampleOS complements the populateSample function by copying into the destinations the fields from the source
// that are exclusive of Darwin Storage Samples
func populateSampleOS(_, _ *Sample) {
	//intentionally left empty, no OS specific values
}

func populateUsageOS(_ *disk.UsageStat, _ *Sample) {
	//intentionally left empty, no OS specific usage values
}

func CalculateDeviceMapping(_ map[string]bool, _ bool) (deviceMap map[string]string) {
	//intentionally left empty, IO per partition not supported yet in darwin
	return
}

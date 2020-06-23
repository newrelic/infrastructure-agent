// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"fmt"

	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var sslog = log.WithComponent("StorageSampler")

// BaseSample provides the basic fields for the storage.Sample instances of all the
// operating systems. The actual storage.Sample types are defined in the corresponding
// OS-bounded code files.
// We use pointers to floats instead of plain floats so that if we don't set one
// of the values, it will not be sent to Dirac. (Not using pointers would mean
// that Go would always send a default value of 0.)
type BaseSample struct {
	sample.BaseEvent

	MountPoint     string `json:"mountPoint"`
	Device         string `json:"device"`
	IsReadOnly     string `json:"isReadOnly"`
	FileSystemType string `json:"filesystemType"`
	CountersSource string `json:"countersSource,omitempty"` // Source for the IOCounters: wmi, pdh, diskstats

	UsedBytes               *float64 `json:"diskUsedBytes,omitempty"`
	UsedPercent             *float64 `json:"diskUsedPercent,omitempty"`
	FreeBytes               *float64 `json:"diskFreeBytes,omitempty"`
	FreePercent             *float64 `json:"diskFreePercent,omitempty"`
	TotalBytes              *float64 `json:"diskTotalBytes,omitempty"`
	TotalUtilizationPercent *float64 `json:"totalUtilizationPercent,omitempty"`
	ReadUtilizationPercent  *float64 `json:"readUtilizationPercent,omitempty"`
	WriteUtilizationPercent *float64 `json:"writeUtilizationPercent,omitempty"`
	ReadBytesPerSec         *float64 `json:"readBytesPerSecond,omitempty"`
	WriteBytesPerSec        *float64 `json:"writeBytesPerSecond,omitempty"`
	ReadWriteBytesPerSecond *float64 `json:"readWriteBytesPerSecond,omitempty"`
	ReadsPerSec             *float64 `json:"readIoPerSecond,omitempty"`
	WritesPerSec            *float64 `json:"writeIoPerSecond,omitempty"`
	IOTimeDelta             uint64   `json:"-"`
	ReadTimeDelta           uint64   `json:"-"`
	WriteTimeDelta          uint64   `json:"-"`
	ReadCountDelta          uint64   `json:"-"`
	WriteCountDelta         uint64   `json:"-"`
	ElapsedSampleDeltaMs    int64    `json:"-"`
	HasDelta                bool     `json:"-"`
}

type PartitionStat struct {
	Device     string `json:"device"`
	Mountpoint string `json:"mountpoint"`
	Fstype     string `json:"fstype"`
	Opts       string `json:"opts"`
}

type IOCountersStat interface {
	fmt.Stringer
	// Source returns an enumerative string of the IO counter source (e.g. "wmi", "pdh", "diskstats"...)
	Source() string
}

type Sampler struct {
	context          agent.AgentContext
	lastRun          time.Time
	lastDiskStats    map[string]IOCountersStat
	lastSamples      sample.EventBatch
	hasBootstrapped  bool
	stopChannel      chan bool
	waitForCleanup   *sync.WaitGroup
	storageUtilities SampleWrapper
	sampleRate       time.Duration
}

type SampleWrapper interface {
	Partitions() ([]PartitionStat, error)
	Usage(path string) (*disk.UsageStat, error)
	IOCounters() (map[string]IOCountersStat, error)
	CalculateSampleValues(counter, lastStats IOCountersStat, elapsedMs int64) *Sample
}

func NewSampler(context agent.AgentContext) *Sampler {
	sampleRateSec := config.DefaultStorageSamplerRateSecs
	if context != nil {
		sampleRateSec = context.Config().MetricsStorageSampleRate
	}

	return &Sampler{
		context:          context,
		waitForCleanup:   &sync.WaitGroup{},
		storageUtilities: NewStorageSampleWrapper(context.Config()),
		sampleRate:       time.Second * time.Duration(sampleRateSec),
	}
}

func (ss *Sampler) useCustomSupportedFileSystems() {
	if ss.context != nil {
		customSupportedFileSystems := ss.context.Config().CustomSupportedFileSystems
		if customSupportedFileSystems != nil && len(customSupportedFileSystems) > 0 {
			var newCustomSupportedFileSystems = map[string]bool{}
			for _, customfs := range customSupportedFileSystems {
				// if custom fs found in list of supported, keep it
				_, found := SupportedFileSystems[customfs]
				if found {
					newCustomSupportedFileSystems[customfs] = true
				}
			}
			// replace original supported fs with new custom set of fs
			SupportedFileSystems = newCustomSupportedFileSystems
			sslog.WithField("supportedFileSystems", SupportedFileSystems).Debug("Using custom supported file systems.")
		}
	}
}

func (ss *Sampler) Interval() time.Duration {
	return ss.sampleRate
}

func (ss *Sampler) Name() string { return "StorageSampler" }

func (ss *Sampler) OnStartup() {
	ss.useCustomSupportedFileSystems()
}

func (ss *Sampler) Disabled() bool {
	return ss.Interval() <= config.FREQ_DISABLE_SAMPLING
}

func (ss *Sampler) Samples() sample.EventBatch {
	return ss.lastSamples
}

func PlatformFsByteScale(b uint64) float64 {
	// Yes, we recognize there could be data loss here
	return float64(b)
}

// Sample samples the storage devices
func (ss *Sampler) Sample() (results sample.EventBatch, err error) {

	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in Sampler.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()
	var cfg *config.Config
	if ss.context != nil {
		cfg = ss.context.Config()
	}

	var elapsedMs int64
	now := time.Now()
	if ss.hasBootstrapped {
		elapsedMs = (now.UnixNano() - ss.lastRun.UnixNano()) / 1000000
	}
	ss.lastRun = now
	ss.hasBootstrapped = true

	partitions, err := ss.storageUtilities.Partitions()
	if err != nil {
		sslog.WithError(err).Error("can't get partitions")
		return nil, err
	}

	var mountPointPrefix string
	if cfg != nil && cfg.IsContainerized {
		mountPointPrefix = cfg.OverrideHostRoot
	}

	//make sure we have a set, not a list
	var activeDevices = map[string]bool{}

	// key: sample deviceKey
	samples := map[string][]*Sample{}
	for _, fs := range partitions {
		helpers.LogStructureDetails(sslog, fs, "Partition", "raw", logrus.Fields{"supported": true})
		sample := &Sample{}
		sample.Type("StorageSample")
		sample.ElapsedSampleDeltaMs = elapsedMs

		// If there is a mountPointPrefix, this means we're most likely running inside a container.
		// Mount points are reported from the perspective of the host. e.g. "/", "/data1"
		//
		// If the host has bind mounted its root to "/host" with associated OverrideHostRoot config,
		// to collect the disk usage we need to resolve the mount points with the host root prefix.
		// e.g. "/" -> "/host" and "/data1" -> "/host/data1"
		mountPoint := filepath.Join(mountPointPrefix, fs.Mountpoint)

		var fsUsage *disk.UsageStat
		if fsUsage, err = ss.storageUtilities.Usage(mountPoint); err != nil {
			sslog.WithError(err).WithField("mountPoint", mountPoint).Warn("can't get disk usage. Ignoring it")
			continue
		}

		helpers.LogStructureDetails(sslog, fsUsage, "PartitionUsage", "raw", nil)

		if cfg != nil && len(cfg.FileDevicesIgnored) > 0 {
			found := false
			fileDevicesIgnored := cfg.FileDevicesIgnored
			sslog.WithField("fileDevicesIgnored", fileDevicesIgnored).Debug("Using file device ignored.")
			for _, deviceName := range fileDevicesIgnored {
				if strings.Contains(fs.Device, deviceName) {
					sslog.WithFieldsF(func() logrus.Fields {
						return logrus.Fields{
							"fileDeviceIgnored": deviceName,
							"skippedDevice":     fs.Device,
						}
					}).Debug("Skipping ignored device.")
					found = true
					break
				}
			}
			if found {
				continue
			}
		}

		sample.FileSystemType = fs.Fstype
		sample.MountPoint = fs.Mountpoint // Ensure we use the reported mount point, not the prefixed one
		sample.Device = fs.Device
		sample.IsReadOnly = "false"
		options := strings.Split(fs.Opts, ",")
		for _, s := range options {
			if s == "ro" {
				sample.IsReadOnly = "true"
				break
			}
		}

		populateUsage(fsUsage, sample)

		// we can have multiple mountpoints for the same device
		samples[fs.Device] = append(samples[fs.Device], sample)

		activeDevices[fs.Device] = true
	}

	// Gather IO stats if the OS supports it
	ioCounters, err := ss.storageUtilities.IOCounters()
	if err != nil {
		sslog.WithError(err).Warn("can't get IOCounters")
		err = nil
	} else {
		helpers.LogStructureDetails(sslog, ioCounters, "DiskIOCounters", "raw", nil)

		if ss.lastDiskStats != nil {
			// This can start using a cache at some point
			deviceToLogical := CalculateDeviceMapping(activeDevices, cfg != nil && cfg.IsContainerized)

			helpers.LogStructureDetails(sslog, deviceToLogical, "CalculateDeviceMappings", "raw", nil)

			for deviceKey, counter := range ioCounters {
				// Check to see whether we have a mapping from device key to device
				if device, ok := deviceToLogical[deviceKey]; ok {
					// Check to see whether we had a previous sample recorded for the same device.
					// Without a previous sample, we can't compute metrics which represent the delta since last sampling.
					if lastStats, ok := ss.lastDiskStats[deviceKey]; ok {
						// Look through all accumulated Sample objects for this device. (There could be multiple
						// objects for the same device if it's mounted in multiple locations.)
						if deviceSamples, ok := samples[device]; ok {
							sslog.WithFieldsF(func() logrus.Fields {
								return logrus.Fields{
									"device":    device,
									"deviceKey": fmt.Sprintf("%+v", deviceKey),
									"counter":   fmt.Sprintf("%+v", counter),
									"lastStats": fmt.Sprintf("%+v", lastStats),
								}
							}).Debug("Sampling disk.")

							ioSample := ss.storageUtilities.CalculateSampleValues(counter, lastStats, elapsedMs)
							// use the same disk data for the multiple mountpoints
							for _, ds := range deviceSamples {
								ds.HasDelta = true
								ds.CountersSource = counter.Source()
								populateSample(ioSample, ds)
							}
						}
					}
				} else {
					sslog.WithFieldsF(func() logrus.Fields {
						return logrus.Fields{
							"device":    device,
							"devicekey": deviceKey,
						}
					}).Debug("No device mapping.")
				}
			}
		}
		ss.lastDiskStats = ioCounters
	}

	for _, s := range samples {
		for _, sample := range s {
			results = append(results, sample)
		}
	}
	ss.lastSamples = results

	for _, sample := range results {
		helpers.LogStructureDetails(sslog, sample.(*Sample), "StorageSample", "final", nil)
	}
	return results, nil
}

// PartitionsCache avoids polling for partitions on each sample, since they do not change so frequently
type PartitionsCache struct {
	ttl             time.Duration
	lastInvocation  time.Time
	lastStat        []PartitionStat
	isContainerized bool
	partitionsFunc  func(_ bool) ([]PartitionStat, error)
}

func (c *PartitionsCache) Get() ([]PartitionStat, error) {
	// not sure this is needed
	now := time.Now()
	if c.lastStat != nil && now.Before(c.lastInvocation.Add(c.ttl)) {
		return c.lastStat, nil
	}
	var err error
	c.lastStat, err = c.refresh()
	if err != nil {
		c.lastStat = nil
	}
	c.lastInvocation = now
	return c.lastStat, err
}

func (c *PartitionsCache) refresh() ([]PartitionStat, error) {
	sslog.Debug("Refreshing partitions cache.")
	return c.partitionsFunc(c.isContainerized)
}

// populateSample copies the calculated data from the source sample into the destination sample.
// It must not populate disk.UsageStat data, as it comes from different sources
func populateSample(source, dest *Sample) {
	dest.TotalUtilizationPercent = source.TotalUtilizationPercent
	dest.ReadUtilizationPercent = source.ReadUtilizationPercent
	dest.WriteUtilizationPercent = source.WriteUtilizationPercent
	dest.ReadsPerSec = source.ReadsPerSec
	dest.WritesPerSec = source.WritesPerSec
	dest.ReadBytesPerSec = source.ReadBytesPerSec
	dest.WriteBytesPerSec = source.WriteBytesPerSec
	dest.ReadWriteBytesPerSecond = calculateReadWriteBytesPerSecond(source.ReadBytesPerSec, source.WriteBytesPerSec)
	dest.IOTimeDelta = source.IOTimeDelta
	dest.ReadTimeDelta = source.ReadTimeDelta
	dest.WriteTimeDelta = source.WriteTimeDelta
	dest.ReadCountDelta = source.ReadCountDelta
	dest.WriteCountDelta = source.WriteCountDelta

	// Fields that are exclusive to a given Operation System
	populateSampleOS(source, dest)
}

func calculateReadWriteBytesPerSecond(readBytesPerSec, writeBytesPerSec *float64) *float64 {

	if readBytesPerSec == nil || writeBytesPerSec == nil {
		return nil
	}

	readWriteBytesPerSecond := *readBytesPerSec + *writeBytesPerSec

	return &readWriteBytesPerSecond
}

// populateUsage copies the Usage Stats inside the destination sample
func populateUsage(fsUsage *disk.UsageStat, dest *Sample) {
	usedBytes := PlatformFsByteScale(fsUsage.Used)
	totalBytes := PlatformFsByteScale(fsUsage.Total)
	freeBytes := PlatformFsByteScale(fsUsage.Free)

	dest.UsedBytes = &usedBytes
	dest.TotalBytes = &totalBytes
	dest.FreeBytes = &freeBytes

	// used percent calculations use total of usedBytes + freeBytes since totalBytes
	// on linux includes space reserved for the operating system
	usedPercent := usedBytes / (usedBytes + freeBytes) * 100
	freePercent := 100 - usedPercent
	dest.UsedPercent = &usedPercent
	dest.FreePercent = &freePercent

	populateUsageOS(fsUsage, dest)
}

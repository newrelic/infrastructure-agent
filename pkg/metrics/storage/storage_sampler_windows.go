// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package storage

import (
	"bytes"
	"syscall"
	"time"
	"unsafe"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/disk"
)

var (
	SupportedFileSystems        = map[string]bool{"NTFS": true, "ReFS": true}
	Modkernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetLogicalDriveStringsW = Modkernel32.NewProc("GetLogicalDriveStringsW")
	procGetDriveType            = Modkernel32.NewProc("GetDriveTypeW")
	provGetVolumeInformation    = Modkernel32.NewProc("GetVolumeInformationW")
	FileFileCompression         = int64(16)     // 0x00000010
	FileReadOnlyVolume          = int64(524288) // 0x00080000
)

type Sample struct {
	BaseSample

	AvgQueueLen      *float64 `json:"avgQueueLen,omitempty"`
	AvgReadQueueLen  *float64 `json:"avgReadQueueLen,omitempty"`
	AvgWriteQueueLen *float64 `json:"avgWriteQueueLen,omitempty"`
	CurrentQueueLen  *float64 `json:"currentQueueLen,omitempty"`
}

type WinStorageSampleWrapper struct {
	legacy      bool
	partitions  PartitionsCache
	pdhCounters PdhIoCounters
}

func (ssw *WinStorageSampleWrapper) Partitions() ([]PartitionStat, error) {
	return ssw.partitions.Get()
}

func (ssw *WinStorageSampleWrapper) Usage(path string) (*disk.UsageStat, error) {
	return disk.Usage(path)
}

func (ssw *WinStorageSampleWrapper) IOCounters() (map[string]IOCountersStat, error) {
	// This will be removed in future agent versions. By now, pdh can be optionally disabled
	if !ssw.legacy {
		partitions, err := ssw.partitions.Get()
		if err != nil {
			sslog.WithError(err).Debug("Fetching partitions.")
		}
		counters, err := ssw.pdhCounters.IoCounters(partitions)
		if err == nil {
			return counters, nil
		}
		sslog.WithError(err).Debug("PDH IoCounters failed. Falling back to WMI")
	}
	return WmiIoCounters()
}

func (ssw *WinStorageSampleWrapper) CalculateSampleValues(counter, lastStats IOCountersStat, elapsedMs int64) *Sample {
	switch counter.(type) {
	case *WmiIoCountersStat:
		// It may happen that lastStats is not WMI, using the counter as lastStats to report only non-deltas
		if _, ok := lastStats.(*WmiIoCountersStat); !ok {
			return CalculateWmiSampleValues(counter.(*WmiIoCountersStat), counter.(*WmiIoCountersStat), elapsedMs)
		}
		return CalculateWmiSampleValues(counter.(*WmiIoCountersStat), lastStats.(*WmiIoCountersStat), elapsedMs)
	case *PdhIoCountersStat:
		return CalculatePdhSampleValues(counter.(*PdhIoCountersStat), nil, elapsedMs)
	}
	panic(errors.Errorf("%#v is neither *WmiIoCountersStat nor (*PdhIoCountersStat)", counter))
}

func NewStorageSampleWrapper(cfg *config.Config) SampleWrapper {
	ttl, err := time.ParseDuration(cfg.PartitionsTTL)
	if err != nil {
		ttl = time.Minute // for tests with an unset ttl
	}
	if cfg.LegacyStorageSampler {
		sslog.Info("Using Legacy WMI Storage Sampler")
	}
	ssw := WinStorageSampleWrapper{
		legacy: cfg.LegacyStorageSampler,
		partitions: PartitionsCache{
			ttl:             ttl,
			isContainerized: cfg != nil && cfg.IsContainerized,
			partitionsFunc:  fetchPartitions(cfg.WinRemovableDrives),
		},
		pdhCounters: PdhIoCounters{},
	}
	return &ssw
}

func CalculateDeviceMapping(activeDevices map[string]bool, _ bool) (deviceMap map[string]string) {
	deviceMap = make(map[string]string)
	for d := range activeDevices {
		deviceMap[d] = d
	}
	return
}

// fetches the partitions from the Win32 API
func fetchPartitions(showRemovable bool) func(_ bool) ([]PartitionStat, error) {
	return func(_ bool) (stats []PartitionStat, e error) {
		return fetch(showRemovable)
	}
}

func fetch(showRemovable bool) ([]PartitionStat, error) {
	var ret []PartitionStat
	lpBuffer := make([]byte, 254)
	diskret, _, err := procGetLogicalDriveStringsW.Call(
		uintptr(len(lpBuffer)),
		uintptr(unsafe.Pointer(&lpBuffer[0])))
	if diskret == 0 {
		return ret, err
	}
	for _, v := range lpBuffer {
		if v >= 65 && v <= 90 {
			path := string(v) + ":"
			typepath, _ := syscall.UTF16PtrFromString(path)
			typeret, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(typepath)))

			// 0: UNKNOWN_DRIVE 2: DRIVE_REMOVABLE 3: DRIVE_FIXED 5: DRIVE_CDROM
			if typeret == 0 {
				return ret, syscall.GetLastError()
			}
			if typeret == 3 || (showRemovable && (typeret == 2 || typeret == 5)) {
				lpVolumeNameBuffer := make([]byte, 256)
				lpVolumeSerialNumber := int64(0)
				lpMaximumComponentLength := int64(0)
				lpFileSystemFlags := int64(0)
				lpFileSystemNameBuffer := make([]byte, 256)
				volpath, _ := syscall.UTF16PtrFromString(string(v) + ":/")
				driveret, _, err := provGetVolumeInformation.Call(
					uintptr(unsafe.Pointer(volpath)),
					uintptr(unsafe.Pointer(&lpVolumeNameBuffer[0])),
					uintptr(len(lpVolumeNameBuffer)),
					uintptr(unsafe.Pointer(&lpVolumeSerialNumber)),
					uintptr(unsafe.Pointer(&lpMaximumComponentLength)),
					uintptr(unsafe.Pointer(&lpFileSystemFlags)),
					uintptr(unsafe.Pointer(&lpFileSystemNameBuffer[0])),
					uintptr(len(lpFileSystemNameBuffer)))
				if driveret == 0 {
					if typeret == 2 || typeret == 5 {
						continue //device is not ready will happen if there is no disk or media in the drive
					}
					sslog.WithError(err).WithField("path", path).Debug("Unable to read volume information.")
					continue
				}
				opts := "rw"
				if lpFileSystemFlags&FileReadOnlyVolume != 0 {
					opts = "ro"
				}
				if lpFileSystemFlags&FileFileCompression != 0 {
					opts += ".compress"
				}

				d := PartitionStat{
					Mountpoint: path,
					Device:     path,
					Fstype:     string(bytes.Replace(lpFileSystemNameBuffer, []byte("\x00"), []byte(""), -1)),
					Opts:       opts,
				}

				if _, supported := SupportedFileSystems[d.Fstype]; !supported {
					continue
				}
				ret = append(ret, d)
			}
		}
	}
	return ret, nil
}

// populateSampleOS complements the populateSample function by copying into the destinations the fields from the source
// that are exclusive of Windows Storage Samples
func populateSampleOS(source, dest *Sample) {
	dest.AvgQueueLen = source.AvgQueueLen
	dest.AvgReadQueueLen = source.AvgReadQueueLen
	dest.AvgWriteQueueLen = source.AvgWriteQueueLen
	dest.CurrentQueueLen = source.CurrentQueueLen
}

// populateUsage copies the Usage Stats inside the destination sample, for those metrics that are exclusive of Windows
func populateUsageOS(fsUsage *disk.UsageStat, dest *Sample) {
}

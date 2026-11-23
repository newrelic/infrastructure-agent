// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/acquire"
	"github.com/shirou/gopsutil/v3/disk"
	log "github.com/sirupsen/logrus"
)

var (
	mountSource          = ""
	SupportedFileSystems = map[string]bool{
		"xfs":      true,
		"btrfs":    true,
		"ext":      true,
		"ext2":     true,
		"ext3":     true,
		"ext4":     true,
		"hfs":      true,
		"vxfs":     true,
		"zfs":      true,
		"reiserfs": true,
	}
	deviceRegexp      = regexp.MustCompile("^/dev/([a-z0-9]+)")
	lvmRegexp         = regexp.MustCompile("^/dev/mapper/(.*)")
	lvmVolumeIdRegexp = regexp.MustCompile("^/dev/mapper/VolGroup[0-9]+-LogVol([0-9]+)$")
	invoke            acquire.Invoker
)

const (
	SectorSize = 512
	mountInfo  = "mountinfo"
	mounts     = "mounts"
	mtab       = "mtab"
	partitions = "partitions"
)

type Sample struct {
	BaseSample
	InodesUsed        *uint64  `json:"inodesUsed,omitempty"`
	InodesFree        *uint64  `json:"inodesFree,omitempty"`
	InodesTotal       *uint64  `json:"inodesTotal,omitempty"`
	InodesUsedPercent *float64 `json:"inodesUsedPercent,omitempty"`
}

// Enhanced from GOPSUtil, Adding Utilization
type LinuxIoCountersStat struct {
	ReadCount               uint64 `json:"readCount"`
	MergedReadCount         uint64 `json:"mergedReadCount"`
	WriteCount              uint64 `json:"writeCount"`
	MergedWriteCount        uint64 `json:"mergedWriteCount"`
	ReadBytes               uint64 `json:"readBytes"`
	WriteBytes              uint64 `json:"writeBytes"`
	ReadTime                uint64 `json:"readTime"`
	WriteTime               uint64 `json:"writeTime"`
	IopsInProgress          uint64 `json:"iopsInProgress"`
	IoTime                  uint64 `json:"ioTime"`
	Name                    string `json:"name"`
	SerialNumber            string `json:"serialNumber"`
	TotalUtilizationPercent uint64 `json:"totalUtilizationPercent"`
	ReadUtilizationPercent  uint64 `json:"readUtilizationPercent"`
	WriteUtilizationPercent uint64 `json:"writeUtilizationPercent"`
}

func (d *LinuxIoCountersStat) String() string {
	s, _ := json.Marshal(*d)
	return string(s)
}

func (*LinuxIoCountersStat) Source() string {
	return "diskstats"
}

type LinuxStorageSampleWrapper struct {
	partitions PartitionsCache
}

func (ssw *LinuxStorageSampleWrapper) Partitions() ([]PartitionStat, error) {
	return ssw.partitions.Get()
}

func (ssw *LinuxStorageSampleWrapper) Usage(path string) (*disk.UsageStat, error) {
	return disk.Usage(path)
}

func (ssw *LinuxStorageSampleWrapper) IOCounters() (map[string]IOCountersStat, error) {
	return fetchIoCounters()
}

func (ssw *LinuxStorageSampleWrapper) CalculateSampleValues(counter, lastStats IOCountersStat, elapsedMs int64) *Sample {
	return CalculateSampleValues(counter, lastStats, elapsedMs)
}

func init() {
	invoke = acquire.Invoke{}
}

// MountInfoStat represents linux mount information.
type MountInfoStat struct {
	mountID     int
	parentID    int
	Device      string
	MountPoint  string
	Root        string
	MajMin      string
	FSType      string
	MountSource string
	Opts        string
}

// BlockDevice represents a linux fixed-sized blocks device
type BlockDevice struct {
	Major  string
	Minor  string
	blocks int
	Name   string
}

func NewStorageSampleWrapper(cfg *config.Config) SampleWrapper {
	ttl, err := time.ParseDuration(cfg.PartitionsTTL)
	if err != nil {
		ttl = time.Minute // for tests with an unset ttl
	}
	ssw := LinuxStorageSampleWrapper{
		partitions: PartitionsCache{
			ttl:             ttl,
			isContainerized: cfg != nil && cfg.IsContainerized,
			partitionsFunc:  fetchPartitions,
		},
	}
	return &ssw
}

// populateSampleOS complements the populateSample function by copying into the destinations the fields from the source
// that are exclusive of Linux Storage Samples
func populateSampleOS(_, _ *Sample) {
}

// populateUsage copies the Usage Stats inside the destination sample, for those metrics that are exclusive of Linux
func populateUsageOS(fsUsage *disk.UsageStat, dest *Sample) {
	dest.InodesFree = &fsUsage.InodesFree
	dest.InodesTotal = &fsUsage.InodesTotal
	dest.InodesUsed = &fsUsage.InodesUsed
	dest.InodesUsedPercent = &fsUsage.InodesUsedPercent
}

func CalculateSampleValues(ioCounter IOCountersStat, ioLastStats IOCountersStat, elapsedMs int64) (ioSample *Sample) {
	counter := ioCounter.(*LinuxIoCountersStat)
	lastStats := ioLastStats.(*LinuxIoCountersStat)

	elapsedSeconds := float64(elapsedMs) / 1000

	result := &Sample{}
	readBytes := acquire.CalculateSafeDelta(counter.ReadBytes, lastStats.ReadBytes, elapsedSeconds)

	writeBytes := acquire.CalculateSafeDelta(counter.WriteBytes, lastStats.WriteBytes, elapsedSeconds)

	ioTimeDelta := counter.IoTime - lastStats.IoTime
	readTimeDelta := counter.ReadTime - lastStats.ReadTime
	writeTimeDelta := counter.WriteTime - lastStats.WriteTime

	readCountDelta := counter.ReadCount - lastStats.ReadCount
	writeCountDelta := counter.WriteCount - lastStats.WriteCount

	if elapsedMs > 0 {
		percentUtilized := float64(ioTimeDelta) / float64(elapsedMs) * 100
		if percentUtilized > 100.0 {
			percentUtilized = 100.0
		}
		readWriteTimeDelta := readTimeDelta + writeTimeDelta
		if readWriteTimeDelta > 0 {
			readPortion := float64(readTimeDelta) / float64(readWriteTimeDelta)
			readUtilizationPercent := percentUtilized * readPortion
			result.ReadUtilizationPercent = &readUtilizationPercent

			writePortion := float64(writeTimeDelta) / float64(readWriteTimeDelta)
			writeUtilizationPercent := percentUtilized * writePortion
			result.WriteUtilizationPercent = &writeUtilizationPercent
		}
		result.TotalUtilizationPercent = &percentUtilized
	}

	readsPerSec := acquire.CalculateSafeDelta(counter.ReadCount, lastStats.ReadCount, elapsedSeconds)
	writesPerSec := acquire.CalculateSafeDelta(counter.WriteCount, lastStats.WriteCount, elapsedSeconds)

	result.ReadBytesPerSec = &readBytes
	result.WriteBytesPerSec = &writeBytes
	result.ReadsPerSec = &readsPerSec
	result.WritesPerSec = &writesPerSec
	result.IOTimeDelta = ioTimeDelta
	result.ReadTimeDelta = readTimeDelta
	result.WriteTimeDelta = writeTimeDelta
	result.ReadCountDelta = readCountDelta
	result.WriteCountDelta = writeCountDelta
	return result
}

func parseMountFile(filename string, line string) (mi MountInfoStat, err error) {
	switch filename {
	case mountInfo:
		return parseMountInfo(line)
	case mounts:
		return parseMounts(line)
	case mtab:
		return parseMtab(line)
	}
	return mi, fmt.Errorf("cannot parse %s unsupported mount file", filename)
}

// parseMtab parses a line read from the /etc/mtab file
func parseMtab(line string) (mi MountInfoStat, err error) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return mi, fmt.Errorf("badly formed /etc/mtab file, expected more than 6 columns")
	}
	// Only MountSource and Device are used in parent functions.
	mi = MountInfoStat{
		Device:      fields[0],
		MountSource: fields[0],
		MountPoint:  fields[1],
		FSType:      fields[2],
		Opts:        fields[3],
	}
	return
}

func parseMountInfo(line string) (mi MountInfoStat, err error) {
	fields := strings.Fields(line)
	mountID, err := strconv.Atoi(fields[0])
	if err != nil {
		sslog.WithError(err).Debug("Can't parse mount ID. Assuming zero.")
	}
	parentID, err := strconv.Atoi(fields[1])
	if err != nil {
		sslog.WithError(err).Debug("Can't parse parent mount ID. Assuming zero.")
	}

	mi = MountInfoStat{
		mountID:    mountID,
		parentID:   parentID,
		MajMin:     fields[2],
		Root:       fields[3],
		MountPoint: fields[4],
		Opts:       fields[5],
	}
	// Need to find where the separator exists since field 7 may have optional number of fields.
	separator := 0
	for i := 6; i < len(fields); i++ {
		if fields[i] == "-" {
			separator = i
			break
		}
	}
	if separator == 0 {
		return mi, fmt.Errorf("badly formed /proc/self/mountinfo file, can't find separator")
	}
	mi.FSType = fields[separator+1]
	mi.MountSource = fields[separator+2]
	mi.Device = fields[separator+2]

	if len(fields) >= separator+4 {
		superOpts := strings.Split(fields[separator+3], ",")
		for _, superOpt := range superOpts {
			if superOpt == "ro" || superOpt == "rw" {
				if mi.Opts != "" {
					mi.Opts += ","
				}
				mi.Opts += superOpt
			}
		}
	}

	return
}

func parseMounts(line string) (mi MountInfoStat, err error) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return mi, fmt.Errorf("unexpected number of fields, expected 6, got %d", len(fields))
	}

	mi = MountInfoStat{
		Device:      fields[0],
		MountSource: fields[0],
		MountPoint:  fields[1],
		FSType:      fields[2],
		Opts:        fields[3],
	}
	return
}

func parsePartitions(line string) (b BlockDevice, err error) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return b, fmt.Errorf("unexpected number of fields, expected 4, got %d", len(fields))
	}

	blocks, err := strconv.Atoi(fields[2])
	if err != nil {
		return b, fmt.Errorf("unexpected number of blocks: %w", err)
	}
	b = BlockDevice{
		Major:  fields[0],
		Minor:  fields[1],
		blocks: blocks,
		Name:   fields[3],
	}
	return
}

func isSupportedFs(fsType string) bool {
	_, supported := SupportedFileSystems[fsType]
	return supported
}

// will the get the result of applying the regexp (full name, short name) and if it's a lvm volume
func isLvmMount(name string) ([]string, bool) {
	match := lvmRegexp.FindStringSubmatch(name)
	if len(match) > 1 {
		return match, true
	}

	return nil, false
}

// check whether device is a rootfs: /dev/root
func isRootFS(name string) bool {
	return strings.Contains(name, "root")
}

// pidForProcMounts returns the pid for querying mount files in /proc/
// When we're running inside a container we need to resolve the mounts file from the
// overridden root specifically at PID 1 from the host, because "self" is a symlink
// to the current process's PID and will get the incorrect mounts file of the container.
func pidForProcMounts(isContainerized bool) string {
	if isContainerized {
		return "1"
	}

	return "self"
}

// the file /proc/partitions file contains a table with major and minor number of devices, their number
// of blocks and the device name in /dev
func partitionsInfo() (devices []BlockDevice) {
	partitionsFilePath := helpers.HostProc(partitions)
	lines, err := acquire.ReadLines(partitionsFilePath)
	// EOF means we read the whole file and we should have "lines".
	if err != nil && err != io.EOF {
		sslog.WithError(err).WithField("partitionsFilePath", partitionsFilePath).Error("can't map partitions file")
		return nil
	}

	for lineno, line := range lines {
		// partitions file contains two initial lines used to define the format of the file and
		// separator with the data
		if lineno < 2 {
			continue
		}
		//fmt.Println(line)
		blockInfo, err := parsePartitions(line)
		if err != nil {
			sslog.WithError(err).WithFieldsF(func() log.Fields {
				return log.Fields{
					"lineno": lineno,
					"line":   line,
				}
			}).Error("can't parse block device info line")
			continue
		}
		devices = append(devices, blockInfo)
	}
	return
}

// deviceMapperInfo returns the mounted devices information. Usually from /proc/pid/mountinfo.
// For old systems like Centos/RHEL 5 that don't have /proc/<pid>/mountinfo
// use /etc/mtab because /proc/<pid>/mounts doesn't display properly lvm
// devices, making it impossible to match against io counters.
// The same logic is applied in fetchPartitions.
func deviceMapperInfo(isContainerized bool) (mounts []MountInfoStat) {
	var mountsFile string
	var mountsFilePath string

	pid := pidForProcMounts(isContainerized)

	// get which mount information source to read
	mountsFilePath, mountsFile = getMountsSource(pid)
	lines, err := acquire.ReadLines(mountsFilePath)
	// EOF means we read the whole file and we should have "lines".
	if err != nil && err != io.EOF {
		sslog.WithError(err).WithField("mountsFilePath", mountsFilePath).Error("can't map devices")
		return nil
	}

	unsupportedMountPoints := []log.Fields{}

	for lineno, line := range lines {
		mountInfo, err := parseMountFile(mountsFile, line)
		if err != nil {
			sslog.WithError(err).WithFieldsF(func() log.Fields {
				return log.Fields{
					"lineno": lineno,
					"line":   line,
				}
			}).Error("can't parse mount info line")
			continue
		}
		// could be optimized to not create the struct in the first place
		if !isSupportedFs(mountInfo.FSType) {
			unsupportedMountPoints = append(unsupportedMountPoints, log.Fields{
				"mountsFile": mountsFile,
				"lineno":     lineno,
				"fs":         mountInfo.FSType,
			})
			continue
		}
		// nil = unsupported fs
		mounts = append(mounts, mountInfo)
	}

	if len(unsupportedMountPoints) > 0 {
		sslog.WithTraceField("mountPoints", unsupportedMountPoints).Debug("Unsupported file systems.")
	}

	return
}

// CalculateDeviceMapping maps devices found in mount information file to diskstats device name format
// "Normal" devices are mapped from /dev/sdxy to sdxy
// LVM devices will are mapped from /dev/mapper/xxx to dm-z where z comes either from
//   - Min in MajMin if we have /proc/self[1]/mountInfo
//   - LogVol[z] if the device is named with VolGroup[x]-LogVol[z]
//
// Mounts in /dev/root are mapped to the actual device name using /proc/partitions
// This mapping will fail if we do not have mountInfo (for example older systems with just /proc/mounts) and the device is not named
// with the above pattern of VolGroup-LogVol. If we find ourselves in this situation we have to refactor this a lot more and use
// other tools to make this mapping instead of relying in the simple mount files
func CalculateDeviceMapping(activeDevices map[string]bool, isContainerized bool) (devToFullDevicePath map[string]string) {
	allMounts := deviceMapperInfo(isContainerized)
	devToFullDevicePath = make(map[string]string)

	for deviceName := range activeDevices {
		_, isLvm := isLvmMount(deviceName)
		if isLvm {
			for _, mi := range allMounts {
				if mi.MountSource != deviceName {
					continue
				}
				var devNumbers []string
				// if we have MajMin, use it, otherwise try with a regex based on the name
				if len(mi.MajMin) > 0 {
					devNumbers = strings.Split(mi.MajMin, ":")
					if len(devNumbers) != 2 {
						continue
					}
				} else {
					devNumbers := lvmVolumeIdRegexp.FindStringSubmatch(deviceName)
					if len(devNumbers) == 0 {
						continue
					}
				}

				// mapped device name. ex: dm-x -> /dev/mapper/xxx
				deviceKey := fmt.Sprintf("dm-%s", devNumbers[1])

				devToFullDevicePath[deviceKey] = deviceName

				break
			}
		} else if isRootFS(deviceName) {
			// disk partitions with major and minor
			devices := partitionsInfo()
		Mounts:
			for _, mi := range allMounts {
				if mi.MountSource != deviceName {
					continue
				}
				devNumbers := strings.Split(mi.MajMin, ":")
				if len(devNumbers) != 2 {
					continue
				}
				for _, d := range devices {
					if d.Major == devNumbers[0] && d.Minor == devNumbers[1] {
						devToFullDevicePath[d.Name] = deviceName
						break Mounts
					}
				}
			}
		} else {
			match := deviceRegexp.FindStringSubmatch(deviceName)
			if len(match) > 1 {
				// short device name. ex: sda1 -> /dev/sda1
				deviceKey := match[1]
				devToFullDevicePath[deviceKey] = deviceName
			}
		}
	}

	return
}

// getMountSource returns the path to the mount info file
func getMountsSource(pid string) (string, string) {
	// check for /proc/<pid>/mountInfo
	if _, err := os.Stat(helpers.HostProc(pid, mountInfo)); err == nil {
		mountSource = helpers.HostProc(pid, mountInfo)
		return mountSource, mountInfo
		// check for /proc/<pid>/mounts
	} else if _, err := os.Stat(helpers.HostProc(pid, mounts)); err == nil {
		mountSource = helpers.HostProc(pid, mounts)
		return mountSource, mounts
		// as last recourse, /etc/mtab. on newer systems is just a link to /proc/mounts
	} else {
		mountSource = helpers.HostEtc(mtab)
		return mountSource, mtab
	}
}

func fetchPartitions(isContainerized bool) ([]PartitionStat, error) {

	mountedDevices := deviceMapperInfo(isContainerized)
	if mountedDevices == nil {
		return nil, errors.New("failed to get mounted devices/partitions")
	}

	partitions := make([]PartitionStat, 0, len(mountedDevices))
	for _, m := range mountedDevices {
		d := PartitionStat{
			Device:     m.Device,
			Mountpoint: m.MountPoint,
			Fstype:     m.FSType,
			Opts:       m.Opts,
		}
		partitions = append(partitions, d)
	}

	return partitions, nil
}

func fetchIoCounters() (map[string]IOCountersStat, error) {
	filename := helpers.HostProc("diskstats")
	lines, err := acquire.ReadLines(filename)
	// EOF means we read the whole file and we should have "lines".
	if err != nil && err != io.EOF {
		sslog.WithError(err).WithField("filename", filename).Error("can't read io counters")
		return nil, err
	}
	ret := make(map[string]IOCountersStat, 0)
	empty := LinuxIoCountersStat{}

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			// malformed line in /proc/diskstats, avoid panic by ignoring.
			continue
		}
		name := fields[2]
		reads, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			return ret, err
		}
		mergedReads, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			return ret, err
		}
		rbytes, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			return ret, err
		}
		rtime, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return ret, err
		}
		writes, err := strconv.ParseUint(fields[7], 10, 64)
		if err != nil {
			return ret, err
		}
		mergedWrites, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return ret, err
		}
		wbytes, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return ret, err
		}
		wtime, err := strconv.ParseUint(fields[10], 10, 64)
		if err != nil {
			return ret, err
		}
		iopsInProgress, err := strconv.ParseUint(fields[11], 10, 64)
		if err != nil {
			return ret, err
		}
		iotime, err := strconv.ParseUint(fields[12], 10, 64)
		if err != nil {
			return ret, err
		}
		d := LinuxIoCountersStat{
			ReadBytes:        rbytes * SectorSize,
			WriteBytes:       wbytes * SectorSize,
			ReadCount:        reads,
			WriteCount:       writes,
			MergedReadCount:  mergedReads,
			MergedWriteCount: mergedWrites,
			ReadTime:         rtime,
			WriteTime:        wtime,
			IopsInProgress:   iopsInProgress,
			IoTime:           iotime,
		}
		if d == empty {
			continue
		}
		d.Name = name

		d.SerialNumber = GetDiskSerialNumber(name)
		ret[name] = &d
	}
	return ret, nil
}

// GetDiskSerialNumber returns Serial Number of given device or empty string
// on error. Name of device is expected, eg. /dev/sda
func GetDiskSerialNumber(name string) string {
	n := fmt.Sprintf("--name=%s", name)
	udevadm, err := exec.LookPath("/sbin/udevadm")
	if err != nil {
		return ""
	}

	out, err := invoke.Command(udevadm, "info", "--query=property", n)

	// does not return error, just an empty string
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		values := strings.Split(line, "=")
		if len(values) < 2 || values[0] != "ID_SERIAL" {
			// only get ID_SERIAL, not ID_SERIAL_SHORT
			continue
		}
		return values[1]
	}
	return ""
}

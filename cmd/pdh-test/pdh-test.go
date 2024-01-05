package main

import (
	"bytes"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/sirupsen/logrus"
	"syscall"
	"unsafe"
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

func main() {
	removableDrives := false
	pdhCounters := storage.PdhIoCounters{}

	partitions, err := fetchPartitions(removableDrives)(false)
	if err != nil {
		logrus.WithError(err).Debug("Fetching partitions.")
	}
	counters, err := pdhCounters.IoCounters(partitions)
	fmt.Println(err)
	if err == nil {
		fmt.Println(counters)
		return
	}
	logrus.WithError(err).Debug("PDH IoCounters failed")
}

func fetchPartitions(showRemovable bool) func(_ bool) ([]storage.PartitionStat, error) {
	return func(_ bool) (stats []storage.PartitionStat, e error) {
		return fetch(showRemovable)
	}
}

func fetch(showRemovable bool) ([]storage.PartitionStat, error) {
	var ret []storage.PartitionStat
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
					logrus.WithError(err).WithField("path", path).Debug("Unable to read volume information.")
					continue
				}
				opts := "rw"
				if lpFileSystemFlags&FileReadOnlyVolume != 0 {
					opts = "ro"
				}
				if lpFileSystemFlags&FileFileCompression != 0 {
					opts += ".compress"
				}

				d := storage.PartitionStat{
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

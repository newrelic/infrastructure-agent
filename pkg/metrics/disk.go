// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"strings"

	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

const (
	dockerMountPoint     = "/var/lib/docker/devicemapper/mnt/"
	kubernetesMountPoint = "/var/lib/kubelet/"
)

type DiskSample struct {
	UsedBytes               float64 `json:"diskUsedBytes"`
	UsedPercent             float64 `json:"diskUsedPercent"`
	FreeBytes               float64 `json:"diskFreeBytes"`
	FreePercent             float64 `json:"diskFreePercent"`
	TotalBytes              float64 `json:"diskTotalBytes"`
	UtilizationPercent      float64 `json:"diskUtilizationPercent"`
	ReadUtilizationPercent  float64 `json:"diskReadUtilizationPercent"`
	WriteUtilizationPercent float64 `json:"diskWriteUtilizationPercent"`
	ReadsPerSec             float64 `json:"diskReadsPerSecond"`
	WritesPerSec            float64 `json:"diskWritesPerSecond"`
}

type DiskMonitor struct {
	storageSampler *storage.Sampler
}

func NewDiskMonitor(storageSampler *storage.Sampler) *DiskMonitor {
	return &DiskMonitor{storageSampler: storageSampler}
}

// filterStorageSamples will be used to remove disk samples that should not be taken into account. e.g. duplicates.
func FilterStorageSamples(samples sample.EventBatch) []*storage.Sample {
	var result []*storage.Sample

	seen := make(map[string]*storage.Sample, len(samples))
	for _, sample := range samples {
		ss, ok := sample.(*storage.Sample)
		if !ok {
			continue
		}
		// Remove duplicated devices.
		if _, ok := seen[ss.Device]; ok {
			continue
		}
		seen[ss.Device] = ss

		if strings.Contains(ss.MountPoint, kubernetesMountPoint) ||
			strings.Contains(ss.MountPoint, dockerMountPoint) {
			continue
		}

		result = append(result, ss)
	}
	return result
}

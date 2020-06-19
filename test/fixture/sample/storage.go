// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fixture

import (
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
)

var storageTraffic = struct {
	Bytes        float64
	Percent      float64
	Delta        uint64
	ElapsedDelta int64
	hasDelta     bool
}{
	121212.1212,
	.8,
	0,
	0,
	false,
}

var StorageSample = storage.Sample{BaseSample: storage.BaseSample{
	BaseEvent: sample.BaseEvent{
		EntityKey: "my-entity-key",
	},
	MountPoint:     "/",
	Device:         "sda1",
	IsReadOnly:     "true",
	FileSystemType: "ext4",

	UsedBytes:               &storageTraffic.Bytes,
	UsedPercent:             &storageTraffic.Percent,
	FreeBytes:               &storageTraffic.Bytes,
	FreePercent:             &storageTraffic.Percent,
	TotalBytes:              &storageTraffic.Bytes,
	TotalUtilizationPercent: &storageTraffic.Percent,
	ReadUtilizationPercent:  &storageTraffic.Percent,
	WriteUtilizationPercent: &storageTraffic.Percent,
	ReadBytesPerSec:         &storageTraffic.Bytes,
	WriteBytesPerSec:        &storageTraffic.Bytes,
	ReadsPerSec:             &storageTraffic.Bytes,
	WritesPerSec:            &storageTraffic.Bytes,
	IOTimeDelta:             storageTraffic.Delta,
	ReadTimeDelta:           storageTraffic.Delta,
	WriteTimeDelta:          storageTraffic.Delta,
	ReadCountDelta:          storageTraffic.Delta,
	WriteCountDelta:         storageTraffic.Delta,
	ElapsedSampleDeltaMs:    storageTraffic.ElapsedDelta,
	HasDelta:                storageTraffic.hasDelta,
}}

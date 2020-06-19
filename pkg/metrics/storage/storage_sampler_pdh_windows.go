// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"encoding/json"
	"fmt"

	"github.com/newrelic/infrastructure-agent/internal/windows"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

type PdhIoCountersStat struct {
	ReadsPerSec      float64
	ReadBytesPerSec  float64
	WritesPerSec     float64
	WriteBytesPerSec float64
	TimePercent      float64
	ReadTimePercent  float64
	WriteTimePercent float64

	AvgQueueLen      float64
	AvgReadQueueLen  float64
	AvgWriteQueueLen float64
	CurrentQueueLen  float64
}

func (d *PdhIoCountersStat) String() string {
	s, _ := json.Marshal(*d)
	return string(s)
}

func (*PdhIoCountersStat) Source() string {
	return "pdh"
}

// Metrics that are going to be queried
var metricsNames = []string{
	"\\LogicalDisk(%s)\\Disk Reads/sec",
	"\\LogicalDisk(%s)\\Disk Read Bytes/sec",
	"\\LogicalDisk(%s)\\Disk Writes/sec",
	"\\LogicalDisk(%s)\\Disk Write Bytes/sec",
	"\\LogicalDisk(%s)\\%% Disk Time",
	"\\LogicalDisk(%s)\\%% Disk Read Time",
	"\\LogicalDisk(%s)\\%% Disk Write Time",

	"\\LogicalDisk(%s)\\Avg. Disk Queue Length",
	"\\LogicalDisk(%s)\\Avg. Disk Read Queue Length",
	"\\LogicalDisk(%s)\\Avg. Disk Write Queue Length",
	"\\LogicalDisk(%s)\\Current Disk Queue Length",
}

// Metric indices in the "metrics" array
const (
	readsSec = iota
	readBytesSec
	writesSec
	writeBytesSec
	timePercent
	readTimePercent
	writeTimePercent

	avgQueueLen
	avgReadQueueLen
	avgWriteQueueLen
	currentQueueLen
)

// PdhIoCounters polls for disk IO counters using the Windows PDH interface
type PdhIoCounters struct {
	started    bool
	pdh        nrwin.PdhPoll
	partitions map[string][]string // key: partition, value: list of metric names for each partition in the system
}

// If the partitions have changed, the PDH query is recreated
func (io *PdhIoCounters) updateQuery(partitions []PartitionStat) error {
	//Checking if the partitions table has changed
	changed := false
	if len(io.partitions) != len(partitions) {
		changed = true
	} else {
		for _, p := range partitions {
			if _, ok := io.partitions[p.Device]; !ok {
				changed = true
				break
			}
		}
	}
	if changed || !io.started {
		sslog.Debug("Creating new PDH query.")
		io.partitions = map[string][]string{}
		metrics := make([]string, 0, len(metricsNames)*len(partitions))
		for _, p := range partitions {
			sslog.WithField("partition", fmt.Sprintf("%#v", p)).Debug("Creating partition queries.")
			io.partitions[p.Device] = make([]string, 0, len(metrics))
			for _, mn := range metricsNames {
				metrics = append(metrics, fmt.Sprintf(mn, p.Device))
			}
		}
		var err error
		if io.started {
			err = io.pdh.Close()
			if err != nil {
				sslog.WithError(err).Debug("Closing PDH")
			}
		}
		io.started = false // If "NewPdhPoll" fails, the PdhPoll must be recreated in the next update
		io.pdh, err = nrwin.NewPdhPoll(log.Debugf, metrics...)
		if err != nil {
			return err
		}
		io.started = true
	}
	return nil
}

func (io *PdhIoCounters) IoCounters(partitions []PartitionStat) (map[string]IOCountersStat, error) {
	err := io.updateQuery(partitions)
	if err != nil {
		return nil, err
	}
	values, err := io.pdh.Poll()
	if err != nil {
		return nil, err
	}
	counters := map[string]IOCountersStat{}
	for _, p := range partitions {
		counters[p.Device] = &PdhIoCountersStat{
			ReadsPerSec:      values[fmt.Sprintf(metricsNames[readsSec], p.Device)],
			ReadBytesPerSec:  values[fmt.Sprintf(metricsNames[readBytesSec], p.Device)],
			WritesPerSec:     values[fmt.Sprintf(metricsNames[writesSec], p.Device)],
			WriteBytesPerSec: values[fmt.Sprintf(metricsNames[writeBytesSec], p.Device)],
			TimePercent:      values[fmt.Sprintf(metricsNames[timePercent], p.Device)],
			ReadTimePercent:  values[fmt.Sprintf(metricsNames[readTimePercent], p.Device)],
			WriteTimePercent: values[fmt.Sprintf(metricsNames[writeTimePercent], p.Device)],

			AvgQueueLen:      values[fmt.Sprintf(metricsNames[avgQueueLen], p.Device)],
			AvgReadQueueLen:  values[fmt.Sprintf(metricsNames[avgReadQueueLen], p.Device)],
			AvgWriteQueueLen: values[fmt.Sprintf(metricsNames[avgWriteQueueLen], p.Device)],
			CurrentQueueLen:  values[fmt.Sprintf(metricsNames[currentQueueLen], p.Device)],
		}
	}
	return counters, nil
}

// CalculatePdhSampleValues return a Sample instance, calculated from a single PdhIoCountersStat
func CalculatePdhSampleValues(s, _ *PdhIoCountersStat, elapsedMs int64) *Sample {
	return &Sample{
		BaseSample: BaseSample{
			ReadsPerSec:             &s.ReadsPerSec,
			ReadBytesPerSec:         &s.ReadBytesPerSec,
			WritesPerSec:            &s.WritesPerSec,
			WriteBytesPerSec:        &s.WriteBytesPerSec,
			TotalUtilizationPercent: &s.TimePercent,
			ReadUtilizationPercent:  &s.ReadTimePercent,
			WriteUtilizationPercent: &s.WriteTimePercent,
			HasDelta:                true,
			WriteCountDelta:         uint64(s.WritesPerSec * float64(elapsedMs) / 1000),
			ReadCountDelta:          uint64(s.ReadsPerSec * float64(elapsedMs) / 1000),
		},
		AvgQueueLen:      &s.AvgQueueLen,
		AvgReadQueueLen:  &s.AvgReadQueueLen,
		AvgWriteQueueLen: &s.AvgWriteQueueLen,
		CurrentQueueLen:  &s.CurrentQueueLen,
	}
}

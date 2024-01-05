// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package storage

import (
	"encoding/json"
	"fmt"

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
	"\\Disco lógico(%s)\\Lecturas de disco/s",
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
			ReadsPerSec: values[fmt.Sprintf(metricsNames[readsSec], p.Device)],
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

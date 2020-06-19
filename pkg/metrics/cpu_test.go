// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"encoding/json"
	"testing"

	"github.com/shirou/gopsutil/cpu"
	"github.com/stretchr/testify/assert"
)

func TestNewCPUMonitor(t *testing.T) {
	m := NewCPUMonitor(nil)

	assert.NotNil(t, m)
}

func TestCPUSample(t *testing.T) {
	m := NewCPUMonitor(nil)

	result, err := m.Sample()

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCpuMarshallableSample_NormalOperation(t *testing.T) {
	cpuTimes := func(_ bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{{"1", 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}}, nil
	}
	cpuMon := CPUMonitor{
		cpuTimes: cpuTimes,
		last:     []cpu.TimesStat{{"1", 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 1.0}},
	}
	sample, err := cpuMon.Sample()
	assert.NoError(t, err)
	_, err = json.Marshal(sample)
	assert.NoError(t, err)
}

func TestCpuMarshallableSample_ZeroDeltas(t *testing.T) {
	cpuTimes := func(_ bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{{"1", 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}}, nil
	}
	cpuMon := CPUMonitor{
		cpuTimes: cpuTimes,
		last:     []cpu.TimesStat{{"1", 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}},
	}
	sample, err := cpuMon.Sample()
	assert.NoError(t, err)
	_, err = json.Marshal(sample)
	assert.NoError(t, err)
}

func TestCPUDelta(t *testing.T) {
	cpu1 := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     1.0,
		GuestNice: 1.0,
		Idle:      1.0,
		Iowait:    1.0,
		Irq:       1.0,
		Nice:      1.0,
		Softirq:   1.0,
		Steal:     1.0,
		System:    1.0,
		User:      1.0,
	}

	cpu2 := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     1.0,
		GuestNice: 1.0,
		Idle:      1.0,
		Iowait:    1.0,
		Irq:       1.0,
		Nice:      1.0,
		Softirq:   1.0,
		Steal:     1.0,
		System:    1.0,
		User:      1.0,
	}

	assert.Equal(t, &cpu.TimesStat{
		CPU:       "intel",
		Guest:     0.0,
		GuestNice: 0.0,
		Idle:      0.0,
		Iowait:    0.0,
		Irq:       0.0,
		Nice:      0.0,
		Softirq:   0.0,
		Steal:     0.0,
		System:    0.0,
		User:      0.0,
	}, cpuDelta(cpu1, cpu2))
}

func TestCPUDelta_NegativeSteal(t *testing.T) {
	current := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     10.0,
		GuestNice: 10.0,
		Idle:      10.0,
		Iowait:    10.0,
		Irq:       10.0,
		Nice:      10.0,
		Softirq:   10.0,
		Steal:     10.0,
		System:    10.0,
		User:      10.0,
	}

	previous := &cpu.TimesStat{
		CPU:       "intel",
		Guest:     1.0,
		GuestNice: 1.0,
		Idle:      1.0,
		Iowait:    1.0,
		Irq:       1.0,
		Nice:      1.0,
		Softirq:   1.0,
		Steal:     100.0,
		System:    1.0,
		User:      1.0,
	}

	assert.Equal(t, &cpu.TimesStat{
		CPU:       "intel",
		Guest:     9.0,
		GuestNice: 9.0,
		Idle:      9.0,
		Iowait:    9.0,
		Irq:       9.0,
		Nice:      9.0,
		Softirq:   9.0,
		Steal:     0.0,
		System:    9.0,
		User:      9.0,
	}, cpuDelta(current, previous))
}

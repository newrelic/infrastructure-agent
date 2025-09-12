//go:build windows
// +build windows

// Copyright 2024 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	nrwin "github.com/newrelic/infrastructure-agent/internal/windows"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

//nolint:gochecknoglobals
var cpuWindowsLog = log.WithComponent("CPUWindows")

// Windows CPU performance counter paths using wildcard for all cores
// Using "Processor Information" instead of "Processor" for better CPU group support
// Following Elastic Agent System Metrics approach
const (
	processorTimeAllCores  = "\\Processor Information(*)\\% Processor Time"
	userTimeAllCores       = "\\Processor Information(*)\\% User Time"
	privilegedTimeAllCores = "\\Processor Information(*)\\% Privileged Time"
	idleTimeAllCores       = "\\Processor Information(*)\\% Idle Time"
	interruptTimeAllCores  = "\\Processor Information(*)\\% Interrupt Time"
	dpcTimeAllCores        = "\\Processor Information(*)\\% DPC Time"
)

type WindowsCPUMonitor struct {
	context            agent.AgentContext
	rawPoll            *nrwin.PdhRawPoll
	started            bool
	requiresTwoSamples bool
	lastSample         map[string][]nrwin.CPUGroupInfo
	lastTimestamp      time.Time
}

// NewCPUMonitor creates a new Windows CPU monitor using Elastic Agent System Metrics proven approach
// This implementation uses PDH's raw counters with manual calculation for reliable CPU monitoring
func NewCPUMonitor(context agent.AgentContext) *CPUMonitor {
	winMonitor := &WindowsCPUMonitor{
		context:            context,
		requiresTwoSamples: true, // PDH requires two samples for rate counters
	}

	return &CPUMonitor{
		context:        context,
		cpuTimes:       nil,
		windowsMonitor: winMonitor,
	}
}

func (w *WindowsCPUMonitor) initializeRawPDH() error {
	if w.started {
		return nil
	}

	var err error
	// Initialize raw PDH poll following Elastic Agent System Metrics approach
	w.rawPoll, err = nrwin.NewPdhRawPoll(
		cpuWindowsLog.Debugf,
		processorTimeAllCores,
		userTimeAllCores,
		privilegedTimeAllCores,
		idleTimeAllCores,
		interruptTimeAllCores,
		dpcTimeAllCores,
	)
	if err != nil {
		return fmt.Errorf("failed to create raw PDH poll: %w", err)
	}

	w.started = true
	return nil
}

func (w *WindowsCPUMonitor) sample() (*CPUSample, error) {
	if err := w.initializeRawPDH(); err != nil {
		return nil, err
	}

	// Get raw counter data following Elastic Agent System Metrics approach
	rawData, err := w.rawPoll.PollRawArray()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw CPU performance counters: %w", err)
	}

	helpers.LogStructureDetails(cpuWindowsLog, rawData, "RawCPUPerfCounters", "raw", nil)

	// Process the raw data following Elastic Agent System Metrics aggregation pattern
	userTimeData := rawData[userTimeAllCores]
	privilegedTimeData := rawData[privilegedTimeAllCores]
	idleTimeData := rawData[idleTimeAllCores]

	if len(userTimeData) == 0 {
		return nil, fmt.Errorf("no user time data available")
	}

	// For the first sample, we need two collections to calculate rates
	if w.requiresTwoSamples || w.lastSample == nil {
		w.lastSample = rawData
		w.lastTimestamp = time.Now()
		w.requiresTwoSamples = false
		// Return zero sample for first collection
		return &CPUSample{
			CPUPercent:       0,
			CPUUserPercent:   0,
			CPUSystemPercent: 0,
			CPUIOWaitPercent: 0,
			CPUIdlePercent:   0,
			CPUStealPercent:  0,
		}, nil
	}

	// Calculate CPU percentages using absolute values following Elastic Agent System Metrics approach
	// Elastic uses: time.Duration(rawData[i].RawValue.FirstValue*100) / time.Millisecond
	currentTimestamp := time.Now()
	timeDelta := currentTimestamp.Sub(w.lastTimestamp)

	var totalUserTime, totalPrivilegedTime, totalIdleTime float64
	var validCoreCount int

	// Aggregate data from all CPU cores following Elastic Agent System Metrics approach
	lastUserTimeData := w.lastSample[userTimeAllCores]
	lastPrivilegedTimeData := w.lastSample[privilegedTimeAllCores]
	lastIdleTimeData := w.lastSample[idleTimeAllCores]

	validCoreCount = w.calculateAbsoluteCounterDeltas(
		userTimeData, lastUserTimeData, timeDelta, &totalUserTime, "user")
	w.calculateAbsoluteCounterDeltas(
		privilegedTimeData, lastPrivilegedTimeData, timeDelta, &totalPrivilegedTime, "privileged")
	w.calculateAbsoluteCounterDeltas(
		idleTimeData, lastIdleTimeData, timeDelta, &totalIdleTime, "idle")

	// Update last sample for next iteration
	w.lastSample = rawData
	w.lastTimestamp = currentTimestamp

	// If no valid data, return zero sample
	if validCoreCount == 0 {
		return &CPUSample{
			CPUPercent:       0,
			CPUUserPercent:   0,
			CPUSystemPercent: 0,
			CPUIOWaitPercent: 0,
			CPUIdlePercent:   0,
			CPUStealPercent:  0,
		}, nil
	}

	// Calculate average percentages across all cores
	avgUserTime := totalUserTime / float64(validCoreCount)
	avgSystemTime := totalPrivilegedTime / float64(validCoreCount)
	avgIdleTime := totalIdleTime / float64(validCoreCount)

	// CPU usage is user + system (everything except idle)
	avgProcessorTime := avgUserTime + avgSystemTime

	// Ensure values are within valid ranges following Elastic Agent System Metrics validation
	avgProcessorTime = normalizePercentage(avgProcessorTime)
	avgUserTime = normalizePercentage(avgUserTime)
	avgSystemTime = normalizePercentage(avgSystemTime)
	avgIdleTime = normalizePercentage(avgIdleTime)

	sample := &CPUSample{
		CPUPercent:       avgProcessorTime,
		CPUUserPercent:   avgUserTime,
		CPUSystemPercent: avgSystemTime,
		CPUIOWaitPercent: 0, // Windows doesn't have a direct equivalent to IOWait
		CPUIdlePercent:   avgIdleTime,
		CPUStealPercent:  0, // Windows doesn't have steal time
	}

	cpuWindowsLog.WithField("sample", sample).Debug("CPU sample calculated using PDH raw counters")

	return sample, nil
}

// calculateAbsoluteCounterDeltas calculates percentage from absolute counter values following Elastic Agent System Metrics pattern
// Elastic approach: time.Duration(rawData[i].RawValue.FirstValue*100) / time.Millisecond
func (w *WindowsCPUMonitor) calculateAbsoluteCounterDeltas(
	currentData []nrwin.CPUGroupInfo,
	lastData []nrwin.CPUGroupInfo,
	timeDelta time.Duration,
	total *float64,
	counterType string,
) int {
	validCount := 0
	lastDataMap := make(map[string]nrwin.CPUGroupInfo)

	// Create map of last data for easy lookup
	for _, lastInfo := range lastData {
		if lastInfo.Name != "_Total" {
			lastDataMap[lastInfo.Name] = lastInfo
		}
	}

	for _, currentInfo := range currentData {
		if currentInfo.Name == "_Total" {
			continue
		}

		lastInfo, exists := lastDataMap[currentInfo.Name]
		if !exists {
			cpuWindowsLog.WithFields(map[string]interface{}{
				"type":     counterType,
				"instance": currentInfo.Name,
			}).Debug("No previous sample for instance")
			continue
		}

		// Convert raw values to milliseconds following Elastic Agent System Metrics approach
		// values are in 100-nanosecond intervals, convert to milliseconds
		currentTimeMs := uint64((currentInfo.RawValue.FirstValue * 100) / int64(time.Millisecond))
		lastTimeMs := uint64((lastInfo.RawValue.FirstValue * 100) / int64(time.Millisecond))

		// Calculate delta in milliseconds
		deltaMs := int64(currentTimeMs - lastTimeMs)
		timeDeltaMs := timeDelta.Milliseconds()

		if timeDeltaMs <= 0 || deltaMs < 0 {
			cpuWindowsLog.WithFields(map[string]interface{}{
				"type":        counterType,
				"instance":    currentInfo.Name,
				"deltaMs":     deltaMs,
				"timeDeltaMs": timeDeltaMs,
			}).Debug("Invalid time delta")
			continue
		}

		// Calculate percentage: (deltaMs / timeDeltaMs) * 100
		percentage := (float64(deltaMs) / float64(timeDeltaMs)) * 100.0

		// Ensure percentage is within valid range
		if percentage < 0 {
			percentage = 0
		} else if percentage > 100 {
			percentage = 100
		}

		*total += percentage
		validCount++

		if cpuWindowsLog.IsDebugEnabled() {
			cpuWindowsLog.WithFields(map[string]interface{}{
				"type":        counterType,
				"instance":    currentInfo.Name,
				"currentMs":   currentTimeMs,
				"lastMs":      lastTimeMs,
				"deltaMs":     deltaMs,
				"timeDeltaMs": timeDeltaMs,
				"percentage":  percentage,
			}).Debug("Absolute counter calculation following Elastic approach")
		}
	}

	return validCount
}

func (w *WindowsCPUMonitor) close() error {
	if w.started && w.rawPoll != nil {
		return w.rawPoll.Close()
	}
	return nil
}

// normalizePercentage ensures percentage values are within valid 0-100 range
func normalizePercentage(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

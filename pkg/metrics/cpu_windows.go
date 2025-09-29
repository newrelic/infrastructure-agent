//go:build windows
// +build windows

// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	nrwin "github.com/newrelic/infrastructure-agent/internal/windows"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

//nolint:gochecknoglobals
var (
	cpuWindowsLog = log.WithComponent("CPUWindows")

	// ErrNoUserTimeData indicates no user time data is available.
	ErrNoUserTimeData = errors.New("no user time data available")
)

// Windows CPU performance counter paths using wildcard for all cores.
const (
	processorTimeAllCores  = "\\Processor Information(*)\\% Processor Time"
	userTimeAllCores       = "\\Processor Information(*)\\% User Time"
	privilegedTimeAllCores = "\\Processor Information(*)\\% Privileged Time"
	idleTimeAllCores       = "\\Processor Information(*)\\% Idle Time"
	interruptTimeAllCores  = "\\Processor Information(*)\\% Interrupt Time"
	dpcTimeAllCores        = "\\Processor Information(*)\\% DPC Time"

	// Constants for calculations.
	percentageMultiplier       = 100.0 // Multiplier for percentage calculations
	maxPercentage              = 100   // Maximum percentage value
	nanosecondConversionFactor = 100   // Converts 100-nanosecond units to nanoseconds
	timestampShiftBits         = 32    // Bits to shift for timestamp calculation
	utf16PointerAdvanceBytes   = 2     // Bytes to advance UTF16 pointer
)

type WindowsCPUMonitor struct {
	context            agent.AgentContext
	rawPoll            *nrwin.PdhRawPoll
	started            bool
	requiresTwoSamples bool
	lastSample         map[string][]nrwin.CPUGroupInfo
	lastTimestamp      time.Time
}

// NewCPUMonitor uses PDH's raw counters with manual calculation for reliable CPU monitoring.
func NewCPUMonitor(context agent.AgentContext) *CPUMonitor {
	winMonitor := &WindowsCPUMonitor{
		context:            context,
		rawPoll:            nil,
		started:            false,
		requiresTwoSamples: true, // PDH requires two samples for rate counters
		lastSample:         nil,
		lastTimestamp:      time.Time{},
	}
	return &CPUMonitor{
		context:        context,
		last:           nil,
		cpuTimes:       nil,
		windowsMonitor: winMonitor,
	}
}

func (w *WindowsCPUMonitor) initializeRawPDH() error {
	if w.started {
		return nil
	}

	var err error
	// Initialize raw PDH poll.
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

	// Get raw counter data
	rawData, err := w.rawPoll.PollRawArray()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw CPU performance counters: %w", err)
	}

	helpers.LogStructureDetails(cpuWindowsLog, rawData, "RawCPUPerfCounters", "raw", nil)

	// Process the raw data
	userTimeData := rawData[userTimeAllCores]
	privilegedTimeData := rawData[privilegedTimeAllCores]
	idleTimeData := rawData[idleTimeAllCores]

	if len(userTimeData) == 0 {
		return nil, fmt.Errorf("failed to get CPU user time data: %w", ErrNoUserTimeData)
	}

	// For the first sample, we need two collections to calculate rates
	if w.requiresTwoSamples || w.lastSample == nil {
		w.lastSample = rawData
		w.lastTimestamp = time.Now()
		w.requiresTwoSamples = false
		// Return zero sample for first collection
		return defaultCPUSample, nil
	}

	// Calculate CPU percentages using delta values
	currentTimestamp := time.Now()

	var totalUserTime, totalPrivilegedTime, totalIdleTime time.Duration

	// Aggregate data from all CPU cores
	lastUserTimeData := w.lastSample[userTimeAllCores]
	lastPrivilegedTimeData := w.lastSample[privilegedTimeAllCores]
	lastIdleTimeData := w.lastSample[idleTimeAllCores]

	validCoreCount := w.calculateCPUTimeDelta(
		userTimeData, lastUserTimeData, &totalUserTime, "user")
	w.calculateCPUTimeDelta(
		privilegedTimeData, lastPrivilegedTimeData, &totalPrivilegedTime, "privileged")
	w.calculateCPUTimeDelta(
		idleTimeData, lastIdleTimeData, &totalIdleTime, "idle")

	// Update last sample for next iteration
	w.lastSample = rawData
	w.lastTimestamp = currentTimestamp

	// If no valid data, return zero sample
	if validCoreCount == 0 {
		return defaultCPUSample, nil
	}

	// Calculate total system CPU time (sum across cores)
	// This represents the total time spent by all CPU cores
	totalSystemTime := totalPrivilegedTime
	totalCPUTime := totalUserTime + totalSystemTime + totalIdleTime

	if totalCPUTime == 0 {
		// Avoid division by zero
		return defaultCPUSample, nil
	}

	// Calculate percentages from the time deltas
	userPercent := calculatePercent(totalUserTime, totalCPUTime)
	systemPercent := calculatePercent(totalSystemTime, totalCPUTime)
	idlePercent := calculatePercent(totalIdleTime, totalCPUTime)

	// CPU usage is user + system (everything except idle)
	cpuUsagePercent := userPercent + systemPercent

	// Ensure values are within valid ranges
	cpuUsagePercent = normalizePercentage(cpuUsagePercent)
	userPercent = normalizePercentage(userPercent)
	systemPercent = normalizePercentage(systemPercent)
	idlePercent = normalizePercentage(idlePercent)

	sample := &CPUSample{
		CPUPercent:       cpuUsagePercent,
		CPUUserPercent:   userPercent,
		CPUSystemPercent: systemPercent,
		CPUIOWaitPercent: 0, // Windows doesn't have a direct equivalent to IOWait
		CPUIdlePercent:   idlePercent,
		CPUStealPercent:  0, // Windows doesn't have steal time
	}

	cpuWindowsLog.WithField("sample", sample).Debug("CPU sample calculated using PDH raw counters")

	return sample, nil
}

// calculateCPUTimeDelta calculates the delta between current and last counter samples.
func (w *WindowsCPUMonitor) calculateCPUTimeDelta(
	currentData []nrwin.CPUGroupInfo,
	lastData []nrwin.CPUGroupInfo,
	total *time.Duration,
	counterType string,
) int {
	validCount := 0

	// Create a map for quick lookups of last sample data
	lastDataMap := make(map[string]nrwin.CPUGroupInfo)
	for _, data := range lastData {
		lastDataMap[data.Name] = data
	}

	for _, currentInfo := range currentData {
		if currentInfo.Name == "_Total" {
			continue
		}

		// Find corresponding last sample
		lastInfo, exists := lastDataMap[currentInfo.Name]
		if !exists {
			continue
		}

		// Calculate delta between current and last sample
		// Windows performance counters are cumulative, so we need the difference
		delta := currentInfo.RawValue.FirstValue - lastInfo.RawValue.FirstValue
		if delta < 0 {
			// Handle counter wrapping - skip this sample
			cpuWindowsLog.WithFields(map[string]interface{}{
				"type":     counterType,
				"instance": currentInfo.Name,
				"current":  currentInfo.RawValue.FirstValue,
				"last":     lastInfo.RawValue.FirstValue,
				"delta":    delta,
			}).Debug("Counter wrapped - skipping sample (current < last)")

			continue
		}

		// Convert delta to time duration
		// idleTime := time.Duration(delta*100) / time.Millisecond
		// The *100 converts from 100-nanosecond units to nanoseconds
		deltaTime := time.Duration(delta * nanosecondConversionFactor)

		// Sum across all cores
		*total += deltaTime
		validCount++

		if cpuWindowsLog.IsDebugEnabled() {
			cpuWindowsLog.WithFields(map[string]interface{}{
				"type":      counterType,
				"instance":  currentInfo.Name,
				"current":   currentInfo.RawValue.FirstValue,
				"last":      lastInfo.RawValue.FirstValue,
				"delta":     delta,
				"deltaTime": deltaTime,
				"totalSum":  *total,
			}).Debug("CPU time delta calculation (summed across cores)")
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

// calculatePercent calculates the percentage of a part relative to the total.
func calculatePercent(part, total time.Duration) float64 {
	if total == 0 {
		return 0
	}

	return (float64(part) / float64(total)) * percentageMultiplier
}

// normalizePercentage ensures percentage values are within valid 0-100 range.
func normalizePercentage(value float64) float64 {
	if value < 0 {
		return 0
	}

	if value > maxPercentage {
		return maxPercentage
	}

	return value
}

//go:build windows
// +build windows

// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nrwin

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	winapi "github.com/newrelic/infrastructure-agent/internal/windows/api"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	timestampShiftBits = 32 // Bits to shift for timestamp calculation
)

//nolint:gochecknoglobals
var rawPollLog = log.WithComponent("PDHRawPoll")

// errInvalidMetricPath indicates that PdhValidatePath rejected a non-wildcard metric.
var errInvalidMetricPath = errors.New("invalid PDH metric path")

// CPUGroupInfo represents raw CPU performance data for a single CPU core or group
type CPUGroupInfo struct {
	Name      string
	RawValue  winapi.PDH_RAW_COUNTER
	Timestamp uint64
}

// PdhRawPoll creates repeatable queries for Windows PDH API using raw counter arrays
// This is specifically designed to handle CPU metrics across multiple CPU groups
type PdhRawPoll struct {
	metrics []string
	// PDH query handler
	queryHandler winapi.PDH_HQUERY
	// Handles for the different metrics/counters to be queried
	counterHandles []winapi.PDH_HCOUNTER

	debugLog func(string, ...interface{})
}

// NewPdhRawPoll creates a PdhRawPoll object for the provided metric names
// This version is designed to work with wildcard counters like \\Processor(*)\\% Processor Time
func NewPdhRawPoll(loggerFunc func(string, ...interface{}), metrics ...string) (*PdhRawPoll, error) {
	var pdh PdhRawPoll
	pdh.debugLog = loggerFunc

	if pdh.debugLog != nil {
		pdh.debugLog("Initializing NewPdhRawPoll with metrics: %v", metrics)
	}

	// Skip validation for wildcard paths since PdhValidatePath doesn't handle them properly.
	for _, metric := range metrics {
		if pdh.debugLog != nil {
			pdh.debugLog("Checking metric path: %s", metric)
		}

		if containsWildcard(metric) {
			if pdh.debugLog != nil {
				pdh.debugLog("Skipping validation for wildcard metric: %s", metric)
			}

			continue
		}

		ret := winapi.PdhValidatePath(metric)
		if ret != winapi.ERROR_SUCCESS {
			if pdh.debugLog != nil {
				pdh.debugLog("Validation failed for non-wildcard metric %s with error %#v", metric, ret)
			}

			//nolint:wrapcheck // We want to wrap with additional context
			return nil, fmt.Errorf("%w: %q (error %#v)", errInvalidMetricPath, metric, ret)
		}

		if pdh.debugLog != nil {
			pdh.debugLog("Validation successful for non-wildcard metric: %s", metric)
		}
	}

	pdh.counterHandles = make([]winapi.PDH_HCOUNTER, len(metrics))
	pdh.queryHandler = winapi.PDH_HQUERY(uintptr(0))

	if pdh.debugLog != nil {
		pdh.debugLog("Opening PDH query")
	}
	ret := winapi.PdhOpenQuery(0, 0, &pdh.queryHandler)
	if ret != winapi.ERROR_SUCCESS {
		if pdh.debugLog != nil {
			pdh.debugLog("Failed to open PDH query with error %#v", ret)
		}
		return nil, fmt.Errorf("opening PDH query (error %#v)", ret)
	}

	if pdh.debugLog != nil {
		pdh.debugLog("Successfully opened PDH query with handle: %v", pdh.queryHandler)
	}

	pdh.metrics = metrics
	for i, metric := range metrics {
		if pdh.debugLog != nil {
			pdh.debugLog("Adding counter %d/%d for metric: %s", i+1, len(metrics), metric)
		}

		ret = winapi.PdhAddEnglishCounterWithWildcards(pdh.queryHandler, metric, uintptr(0), &pdh.counterHandles[i])
		if ret != winapi.ERROR_SUCCESS {
			if pdh.debugLog != nil {
				pdh.debugLog("Failed to add counter for %s with error %#v (error name: %s)", metric, ret, getErrorName(ret))
			}
			return nil, fmt.Errorf("adding counter for %q (error %#v)", metric, ret)
		}

		if pdh.debugLog != nil {
			pdh.debugLog("Successfully added counter for %s with handle: %v", metric, pdh.counterHandles[i])
		}
	}

	if pdh.debugLog != nil {
		pdh.debugLog("Successfully created PdhRawPoll for %d metrics", len(metrics))
	}

	return &pdh, nil
}

// containsWildcard checks if a counter path contains wildcard characters.
func containsWildcard(path string) bool {
	return strings.Contains(path, "*") || strings.Contains(path, "?")
}

// getErrorName returns a human-readable error name for common PDH error codes.
func getErrorName(errorCode uint32) string {
	switch errorCode {
	case winapi.PDH_CSTATUS_NO_OBJECT:
		return "PDH_CSTATUS_NO_OBJECT"
	case winapi.PDH_CSTATUS_NO_COUNTER:
		return "PDH_CSTATUS_NO_COUNTER"
	case winapi.PDH_CSTATUS_INVALID_DATA:
		return "PDH_CSTATUS_INVALID_DATA"
	case winapi.PDH_MEMORY_ALLOCATION_FAILURE:
		return "PDH_MEMORY_ALLOCATION_FAILURE"
	case winapi.PDH_INVALID_HANDLE:
		return "PDH_INVALID_HANDLE"
	case winapi.PDH_INVALID_ARGUMENT:
		return "PDH_INVALID_ARGUMENT"
	case winapi.PDH_MORE_DATA:
		return "PDH_MORE_DATA"
	case winapi.ERROR_SUCCESS:
		return "ERROR_SUCCESS"
	default:
		return fmt.Sprintf("UNKNOWN_ERROR_%#v", errorCode)
	}
}

// PollRawArray returns raw counter values for all instances of wildcard counters
// This is particularly useful for CPU metrics where you need data from all cores/groups
func (pdh *PdhRawPoll) PollRawArray() (map[string][]CPUGroupInfo, error) {
	rawPollLog.Debug("raw polling start")
	ret := winapi.PdhCollectQueryData(pdh.queryHandler)
	if ret != winapi.ERROR_SUCCESS {
		return nil, fmt.Errorf("collect query returned with %#v", ret)
	}

	results := map[string][]CPUGroupInfo{}

	for counterIndex, cHandle := range pdh.counterHandles {
		var bufferSize uint32
		var bufferCount uint32

		// First call to get buffer size
		ret = winapi.PdhGetRawCounterArray(cHandle, &bufferSize, &bufferCount, nil)
		if ret != winapi.PDH_MORE_DATA && ret != winapi.ERROR_SUCCESS {
			if pdh.debugLog != nil {
				pdh.debugLog("Error getting buffer size for %s (error %#v)", pdh.metrics[counterIndex], ret)
			}
			continue
		}

		if bufferSize == 0 || bufferCount == 0 {
			if pdh.debugLog != nil {
				pdh.debugLog("No data available for %s", pdh.metrics[counterIndex])
			}

			continue
		}

		// Allocate buffer for the raw counter array
		buffer := make([]byte, bufferSize)
		itemBuffer := (*winapi.PDH_RAW_COUNTER_ITEM)(unsafe.Pointer(&buffer[0]))

		// Second call to get actual data
		ret = winapi.PdhGetRawCounterArray(cHandle, &bufferSize, &bufferCount, itemBuffer)
		if ret != winapi.ERROR_SUCCESS {
			if pdh.debugLog != nil {
				pdh.debugLog("Error getting raw counter array for %s (error %#v)", pdh.metrics[counterIndex], ret)
			}

			continue
		}

		// Parse the returned data
		var cpuInfos []CPUGroupInfo
		itemSize := unsafe.Sizeof(winapi.PDH_RAW_COUNTER_ITEM{})

		for itemIndex := range bufferCount {
			// Calculate offset for each item in the array
			offset := uintptr(itemIndex) * itemSize
			item := (*winapi.PDH_RAW_COUNTER_ITEM)(unsafe.Pointer(uintptr(unsafe.Pointer(itemBuffer)) + offset))

			if item.SzName != nil {
				name := winapi.UTF16PtrToString(item.SzName)
				// Use constant instead of magic number 32
				timestamp := uint64(item.RawValue.TimeStamp.HighDateTime)<<timestampShiftBits | uint64(item.RawValue.TimeStamp.LowDateTime)

				cpuInfos = append(cpuInfos, CPUGroupInfo{
					Name:      name,
					RawValue:  item.RawValue,
					Timestamp: timestamp,
				})
			}
		}

		results[pdh.metrics[counterIndex]] = cpuInfos
	}

	rawPollLog.WithField("results", len(results)).Debug("raw polling end")

	return results, nil
}

// Close frees the associated resources and handlers for a PDH query
func (pdh *PdhRawPoll) Close() error {
	ret := winapi.PdhCloseQuery(pdh.queryHandler)
	if ret != winapi.ERROR_SUCCESS {
		return fmt.Errorf("closing query handler (error %#v)", ret)
	}
	return nil
}

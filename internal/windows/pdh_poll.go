// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build windows
// +build windows

// package nrwin stores NewRelic-crafted windows functionalities and tools
package nrwin

import (
	"fmt"

	winapi "github.com/newrelic/infrastructure-agent/internal/windows/api"
	"github.com/newrelic/infrastructure-agent/pkg/log" //nolint:depguard
)

//nolint:gochecknoglobals
var plog = log.WithComponent("PDHPoll")

// PdhPoll creates repeatable queries for the Windows PDH api
type PdhPoll struct {
	metrics []string
	// PDH query handler
	queryHandler winapi.PDH_HQUERY
	// Handles for the different metrics/counters to be queried
	counterHandles []winapi.PDH_HCOUNTER

	debugLog func(string, ...interface{})
}

// NewPdhPoll creates a PdhPoll object for the provided metric names. It also accepts a nillable printf-like function
// to debub some non-critical, wrong returned values
func NewPdhPoll(loggerFunc func(string, ...interface{}), metrics ...string) (PdhPoll, error) {
	var pdh PdhPoll
	pdh.debugLog = loggerFunc

	// Validating that the passed metrics exist
	for _, metric := range metrics {
		ret := winapi.PdhValidatePath(metric)
		if winapi.ERROR_SUCCESS != ret {
			return pdh, fmt.Errorf("with path %q (error %#v)", metric, ret)
		}
	}

	pdh.counterHandles = make([]winapi.PDH_HCOUNTER, len(metrics))
	pdh.queryHandler = winapi.PDH_HQUERY(uintptr(0))
	ret := winapi.PdhOpenQuery(0, 0, &pdh.queryHandler)
	if ret != winapi.ERROR_SUCCESS {
		return pdh, fmt.Errorf("opening PDH query (error %#v)", ret)
	}

	pdh.metrics = metrics
	for i, metric := range metrics {
		ret = winapi.PdhAddEnglishCounter(pdh.queryHandler, metric, uintptr(0), &pdh.counterHandles[i])
		if ret != winapi.ERROR_SUCCESS {
			return pdh, fmt.Errorf("adding counter for %q (error %#v)", metric, ret)
		}
	}

	return pdh, nil
}

// At the moment, the poller is limited to metrics that can be represented as a float64
func (pdh *PdhPoll) Poll() (map[string]float64, error) {
	plog.Debug("polling start")
	ret := winapi.PdhCollectQueryData(pdh.queryHandler)
	if ret != winapi.ERROR_SUCCESS {
		return nil, fmt.Errorf("collect query returned with %#v", ret)
	}

	counters := map[string]float64{}
	var perf winapi.PDH_FMT_COUNTERVALUE_DOUBLE
	for i, cHandle := range pdh.counterHandles {
		ret = winapi.PdhGetFormattedCounterValueDouble(cHandle, nil, &perf)
		if ret != winapi.ERROR_SUCCESS {
			if pdh.debugLog != nil {
				pdh.debugLog("Error getting counter value for %s (error %#v)", pdh.metrics[i], ret)
			}
			continue
		}
		if perf.CStatus != winapi.ERROR_SUCCESS {
			if pdh.debugLog != nil {
				pdh.debugLog("Invalid counter value for %s (status %#v)", pdh.metrics[i], ret)
			}
			continue
		}
		counters[pdh.metrics[i]] = perf.DoubleValue
	}

	plog.WithField("counters", counters).Debug("polling end")

	return counters, nil
}

// Close frees the associated resources a handlers for a PDH query
func (pdh *PdhPoll) Close() error {
	ret := winapi.PdhCloseQuery(pdh.queryHandler)
	if ret != winapi.ERROR_SUCCESS {
		return fmt.Errorf("closing query handler (error %#v)", ret)
	}
	return nil
}

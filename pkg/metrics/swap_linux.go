// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package metrics

import "github.com/shirou/gopsutil/v3/mem"

// returns the available swap metrics for linux OSs
func swapMemory() (*SwapSample, error) {
	swap, err := mem.SwapMemory()
	if err != nil {
		return nil, err
	}

	floatToReference := func(value float64) *float64 {
		return &value
	}

	return &SwapSample{
		SwapFree:  float64(swap.Free),
		SwapTotal: float64(swap.Total),
		SwapUsed:  float64(swap.Used),
		SwapIn:    floatToReference(float64(swap.Sin)),
		SwapOut:   floatToReference(float64(swap.Sout)),
	}, nil
}

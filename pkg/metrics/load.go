// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package metrics

type LoadSample struct {
	LoadOne     float64 `json:"loadAverageOneMinute"`
	LoadFive    float64 `json:"loadAverageFiveMinute"`
	LoadFifteen float64 `json:"loadAverageFifteenMinute"`
}

type LoadMonitor struct {
}

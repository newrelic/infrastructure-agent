// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build darwin

package storage

var (
	SupportedFileSystems = map[string]bool{}
)

func IOCounters() (map[string]IOCountersStat, error) {
	return nil, nil
}

func CalculateDeviceMapping(activeDevices map[string]bool, _ bool) map[string]string {
	return nil
}

func CalculateSampleValues(*IOCountersStat, *IOCountersStat, int64) *Sample {
	return nil
}

func Partitions(supportedFS map[string]bool, isContainerized bool) ([]PartitionStat, error) {
	return nil, nil
}

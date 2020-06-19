// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package metrics

import (
	"fmt"
	"runtime/debug"

	"github.com/shirou/gopsutil/load"
)

func NewLoadMonitor() *LoadMonitor {
	return &LoadMonitor{}
}

func (self *LoadMonitor) Sample() (sample *LoadSample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in LoadMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	load, err := load.Avg()
	if err != nil {
		return nil, err
	}

	return &LoadSample{
		LoadOne:     load.Load1,
		LoadFive:    load.Load5,
		LoadFifteen: load.Load15,
	}, nil

}

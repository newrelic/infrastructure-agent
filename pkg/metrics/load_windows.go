// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package metrics

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
)

var loadOne uint32
var loadFive uint32
var loadFifteen uint32

// implement Linux like load algorithm
const (
	FSHIFT       = 11
	FIXED_1      = (1 << FSHIFT)
	RECOVER_FREQ = 60000  // 5 sec as millis
	LOAD_FREQ    = 5000   // 5 sec as millis
	EXP_1        = 1884   // 1/exp (5s/1m) as fixed point
	EXP_5        = 2014   // 1/exp (5s/5m) as fixed point
	EXP_15       = 2037   // 1/exp (5s/15m) as fixed point
	DIV          = 2048.0 // FIXED_1 as float
	LOAD_FLOOR   = 0.0001
)

func calcAllLoadsLoop() {
	syslog.Debug("Initializing load calculator for Windows.")
	for {
		err := calcAllLoads()
		if err != nil {
			time.Sleep(RECOVER_FREQ * time.Millisecond)
			continue
		}
		time.Sleep(LOAD_FREQ * time.Millisecond)
	}
}

func NewLoadMonitor() *LoadMonitor {
	go calcAllLoadsLoop()
	return &LoadMonitor{}
}

func (self *LoadMonitor) Sample() (sample *LoadSample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in LoadMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	one := loadFloor(float64(loadOne) / DIV)
	five := loadFloor(float64(loadFive) / DIV)
	fifteen := loadFloor(float64(loadFifteen) / DIV)
	return &LoadSample{
		LoadOne:     one,
		LoadFive:    five,
		LoadFifteen: fifteen,
	}, nil
}

func loadFloor(v float64) float64 {
	if v < LOAD_FLOOR {
		return 0
	}
	return v
}

func calcLoad(load, exp, n uint32) uint32 {
	if n > 0 {
		n = n * FIXED_1
	} else {
		n = 0
	}
	result := load * exp
	result = result + (n * (FIXED_1 - exp))
	return (result >> FSHIFT)
}
func calcAllLoads() error {
	pql, err := processQueueLength()
	if err == nil {
		loadOne = calcLoad(loadOne, EXP_1, uint32(pql))
		loadFive = calcLoad(loadFive, EXP_5, uint32(pql))
		loadFifteen = calcLoad(loadFifteen, EXP_15, uint32(pql))
	}
	return err
}

type Win32_PerfFormattedDataOS struct {
	Processes            uint64
	ProcessorQueueLength uint64
	Threads              uint64
}

func processQueueLength() (counter uint64, err error) {
	var dst []Win32_PerfFormattedDataOS

	err = wmi.QueryNamespace("SELECT Processes, ProcessorQueueLength, Threads FROM Win32_PerfFormattedData_PerfOS_System ", &dst,
		config.DefaultWMINamespace)
	if err != nil {
		syslog.WithError(err).Error("getting processor queue stats")
		return 0, err
	}
	// Get last sample if more than one
	for _, d := range dst {
		counter = d.ProcessorQueueLength
	}
	return counter, nil
}

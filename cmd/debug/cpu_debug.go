package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/shirou/gopsutil/v3/cpu"
	"runtime/debug"
	"time"
)

var matcher = func(interface{}) bool { return true }

var syslog = log.WithComponent("SystemSampler")

var last []cpu.TimesStat

type CPUSample struct {
	Core             string
	CPUPercent       float64 `json:"cpuPercent"`
	CPUUserPercent   float64 `json:"cpuUserPercent"`
	CPUSystemPercent float64 `json:"cpuSystemPercent"`
	CPUIOWaitPercent float64 `json:"cpuIOWaitPercent"`
	CPUIdlePercent   float64 `json:"cpuIdlePercent"`
	CPUStealPercent  float64 `json:"cpuStealPercent"`
}

func main() {
	interval := flag.Duration("interval", 30*time.Second, "Interval between samples")
	flag.Parse()

	cpuSample, err := Sample()
	if err != nil {
		panic(err)
	}
	_, err = json.Marshal(cpuSample)
	if err != nil {
		panic(err)
	}

	for {
		cpuSample, err = Sample()
		if err != nil {
			panic(err)
		}
		marshaled, err := json.Marshal(cpuSample)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(marshaled))
		time.Sleep(*interval)
	}
}

func Sample() (sample []*CPUSample, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in CPUMonitor.Sample: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()

	if last == nil {
		last, err = cpu.Times(true)
		return nil, nil
	}

	currentTimes, err := cpu.Times(true)

	// in container envs we might get an empty array and the code panics after this
	if len(currentTimes) <= 0 {
		return nil, nil
	}

	for core, time := range currentTimes {
		delta := cpuDelta(&time, &last[core])

		userDelta := delta.User + delta.Nice
		systemDelta := delta.System + delta.Irq + delta.Softirq
		stolenDelta := delta.Steal

		// Determine percentage values by dividing the total CPU time by each portion, then multiply by 100 to get a percentage from 0-100.
		var userPercent, stolenPercent, systemPercent, ioWaitPercent float64

		deltaTotal := delta.Total()
		if deltaTotal != 0 {
			userPercent = userDelta / deltaTotal * 100.0
			stolenPercent = stolenDelta / deltaTotal * 100.0
			systemPercent = systemDelta / deltaTotal * 100.0
			ioWaitPercent = delta.Iowait / deltaTotal * 100.0
		}
		idlePercent := 100 - userPercent - systemPercent - ioWaitPercent - stolenPercent

		sample = append(sample, &CPUSample{
			Core:             delta.CPU,
			CPUPercent:       userPercent + systemPercent + ioWaitPercent + stolenPercent,
			CPUUserPercent:   userPercent,
			CPUSystemPercent: systemPercent,
			CPUIOWaitPercent: ioWaitPercent,
			CPUIdlePercent:   idlePercent,
			CPUStealPercent:  stolenPercent,
		},
		)
	}
	last = currentTimes

	return
}

func cpuDelta(current, previous *cpu.TimesStat) *cpu.TimesStat {
	var result cpu.TimesStat

	result.CPU = current.CPU
	result.Guest = current.Guest - previous.Guest
	result.GuestNice = current.GuestNice - previous.GuestNice
	result.Idle = current.Idle - previous.Idle
	result.Iowait = current.Iowait - previous.Iowait
	result.Irq = current.Irq - previous.Irq
	result.Nice = current.Nice - previous.Nice
	result.Softirq = current.Softirq - previous.Softirq

	result.Steal = current.Steal - previous.Steal
	// Fixes a bug in some paravirtualized environments that caused steal time decreasing during migrations
	// https://0xstubs.org/debugging-a-flaky-cpu-steal-time-counter-on-a-paravirtualized-xen-guest/
	// https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=871608
	// https://lkml.org/lkml/2017/10/10/182
	if result.Steal < 0 {
		result.Steal = 0
	}
	result.System = current.System - previous.System
	result.User = current.User - previous.User
	return &result
}

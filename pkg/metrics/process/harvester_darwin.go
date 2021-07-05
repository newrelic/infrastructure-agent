// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package process Package process provides all the tools and functionality for sampling processes. It is divided in three main
// components:
// - Snapshot: provides OS-level information of a process at a given spot
// - Harvester: manages process Snapshots to create actual Process Samples with the actual metrics.
// - Sampler: uses the harvester to coordinate the creation of the Process Samples dataset, as being reported to NR
package process

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
	"github.com/sirupsen/logrus"
)

var mplog = log.WithField("component", "Metrics Process")

// Harvester manages sampling for individual processes. It is used by the Process Sampler to get information about the
// existing processes.
type Harvester interface {
	// Pids return the IDs of all the processes that are currently running
	Pids() ([]int32, error)
	// Do performs the actual harvesting operation, returning a process sample containing all the metrics data
	// for the last elapsedSeconds
	Do(pid int32, elapsedSeconds float64) (*types.ProcessSample, error)
}

func newHarvester(ctx agent.AgentContext) *darwinHarvester {
	cfg := ctx.Config()
	// If not config, assuming root mode as default
	privileged := cfg == nil || cfg.RunMode == config.ModeRoot || cfg.RunMode == config.ModePrivileged
	disableZeroRSSFilter := cfg != nil && cfg.DisableZeroRSSFilter
	stripCommandLine := (cfg != nil && cfg.StripCommandLine) || (cfg == nil && config.DefaultStripCommandLine)

	return &darwinHarvester{
		privileged:           privileged,
		disableZeroRSSFilter: disableZeroRSSFilter,
		stripCommandLine:     stripCommandLine,
		serviceForPid:        ctx.GetServiceForPid,
	}
}

// darwinHarvester is a Harvester implementation that uses various darwin sources and manages process caches
type darwinHarvester struct {
	privileged           bool
	disableZeroRSSFilter bool
	stripCommandLine     bool
	serviceForPid        func(int) (string, bool)
}

var _ Harvester = (*darwinHarvester)(nil) // static interface assertion

// Pids returns a slice of process IDs that are running now
func (*darwinHarvester) Pids() ([]int32, error) {
	return process.Pids()
}

// Do Returns a sample of a process whose PID is passed as argument. The 'elapsedSeconds' argument represents the
// time since this process was sampled for the last time. If the process has been sampled for the first time, this value
// will be ignored
func (ps *darwinHarvester) Do(pid int32, _ float64) (*types.ProcessSample, error) {

	var err error
	proc, err := getDarwinProcess(pid, ps.privileged)
	if err != nil {
		return nil, errors.Wrap(err, "can't create process")
	}

	// We don't need to report processes which are not using memory. This filters out certain kernel processes.
	if !ps.disableZeroRSSFilter && proc.VmRSS() == 0 {
		return nil, errors.New("process with zero rss")
	}

	// Creates a fresh process sample and populates it with the metrics data
	sample := metrics.NewProcessSample(pid)

	if err = ps.populateStaticData(sample, proc); err != nil {
		return nil, errors.Wrap(err, "can't populate static attributes")
	}

	if err = ps.populateGauges(sample, proc); err != nil {
		return nil, errors.Wrap(err, "can't fetch gauge data")
	}

	// This must happen every time, even if we already had a cached sample for the process, because
	// the available process name metadata may have changed underneath us (if we pick up a new
	// service/PID association, etc)
	sample.ProcessDisplayName = ps.determineProcessDisplayName(sample)
	sample.Type("ProcessSample")

	return sample, nil
}

// populateStaticData populates the sample with the process data won't vary during the process life cycle
func (ps *darwinHarvester) populateStaticData(sample *types.ProcessSample, process Snapshot) error {
	var err error

	sample.CmdLine, err = process.CmdLine(!ps.stripCommandLine)
	if err != nil {
		return errors.Wrap(err, "acquiring command line")
	}

	sample.User, err = process.Username()
	if err != nil {
		mplog.WithError(err).WithField("processID", sample.ProcessID).Debug("Can't get Username for process.")
	}

	sample.ProcessID = process.Pid()
	sample.CommandName = process.Command()
	sample.ParentProcessID = process.Ppid()

	return nil
}

// populateGauges populates the sample with gauge data that represents the process state at a given point
func (ps *darwinHarvester) populateGauges(sample *types.ProcessSample, process Snapshot) error {
	var err error

	cpuTimes, err := process.CPUTimes()
	if err != nil {
		return err
	}
	sample.CPUPercent = cpuTimes.Percent

	totalCPU := cpuTimes.User + cpuTimes.System

	if totalCPU > 0 {
		sample.CPUUserPercent = (cpuTimes.User / totalCPU) * sample.CPUPercent
		sample.CPUSystemPercent = (cpuTimes.System / totalCPU) * sample.CPUPercent
	} else {
		sample.CPUUserPercent = 0
		sample.CPUSystemPercent = 0
	}

	// Extra status data
	sample.Status = process.Status()
	sample.ThreadCount = process.NumThreads()
	sample.MemoryVMSBytes = process.VmSize()
	sample.MemoryRSSBytes = process.VmRSS()

	return nil
}

// determineProcessDisplayName generates a human-friendly name for this process. By default, we use the command name.
// If we know of a service for this pid, that'll be the name.
func (ps *darwinHarvester) determineProcessDisplayName(sample *types.ProcessSample) string {
	displayName := sample.CommandName
	if serviceName, ok := ps.serviceForPid(int(sample.ProcessID)); ok && len(serviceName) > 0 {
		mplog.WithFieldsF(func() logrus.Fields {
			return logrus.Fields{"serviceName": serviceName, "displayName": displayName, "ProcessID": sample.ProcessID}
		}).Debug("Using service name as display name.")
		displayName = serviceName
	}

	return displayName
}

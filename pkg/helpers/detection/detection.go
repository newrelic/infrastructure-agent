// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package detection

import (
	"errors"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/shirou/gopsutil/process"
)

const InfraAgentProcessName = "newrelic-infra"

// GetInfraAgentProcess returns the pid for the infra-agent process.
func GetInfraAgentProcess() (int32, error) {
	ps, _ := process.Processes()
	for _, p := range ps {
		n, _ := p.Name()

		if n == InfraAgentProcessName {
			return p.Pid, nil
		}
	}
	return 0, errors.New("couldn't find the newrelic-infra process")
}

// IsContainerized is checking if a pid is running inside a docker container.
func IsContainerized(pid int32, dockerAPIVersion string) (isContainerized bool, containerID string, err error) {
	p := &types.ProcessSample{
		ProcessID: pid,
	}
	d := metrics.NewDockerSampler(60, dockerAPIVersion)
	logger := log.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"action":    "IsContainerized",
			"dockerAPI": dockerAPIVersion,
			"pid":       pid,
		}
	})
	if d.Enabled() {
		logger.Info("Docker is enabled, checking for containerized agent")
		var dc metrics.ProcessDecorator
		dc, err = d.NewDecorator()
		if err != nil {
			return
		}
		dc.Decorate(p)
	}

	logger.WithField("containerID", p.ContainerID).Info("Containerized agent found in container")

	isContainerized = p.Contained == "true"
	containerID = p.ContainerID

	return
}

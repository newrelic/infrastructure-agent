// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package detection

import (
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	"github.com/sirupsen/logrus"

	"github.com/shirou/gopsutil/v3/process"
)

const InfraAgentProcessName = "newrelic-infra"

// GetInfraAgentProcess returns the pid for the infra-agent process.
func GetInfraAgentProcess() (int32, error) {
	return GetProcessID(InfraAgentProcessName)
}

// GetProcessID returns the pid for the given process.
func GetProcessID(processName string) (int32, error) {
	ps, _ := process.Processes()
	for _, p := range ps {
		n, _ := p.Name()

		if n == processName {
			return p.Pid, nil
		}
	}

	return 0, fmt.Errorf("couldn't find the %s process", processName) //nolint:goerr113,wrapcheck
}

// IsContainerized is checking if a pid is running inside a docker container.
func IsContainerized(pid int32, dockerAPIVersion, dockerContainerdNamespace string) (isContainerized bool, containerID string, err error) {
	p := &types.ProcessSample{
		ProcessID: pid,
	}
	containerSamplers := metrics.GetContainerSamplers(60, dockerAPIVersion, dockerContainerdNamespace) //nolint:gomnd

	logger := log.WithFieldsF(func() logrus.Fields {
		return logrus.Fields{
			"action":    "IsContainerized",
			"dockerAPI": dockerAPIVersion,
			"pid":       pid,
		}
	})

	for _, containerSampler := range containerSamplers {
		if containerSampler.Enabled() {
			logger.Info("A container runtime is enabled, checking for containerized agent")
			var dc metrics.ProcessDecorator
			dc, err = containerSampler.NewDecorator()
			if err != nil {
				return
			}
			dc.Decorate(p)
		}
	}

	logger.WithField("containerID", p.ContainerID).Info("Containerized agent found in container")

	isContainerized = p.Contained == "true"
	containerID = p.ContainerID

	return
}

// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
)

var (
	ErrNoCloudHostTypeNotAvailable = fmt.Errorf("unable to retrieve host type, cloud harvester not available")
)

type HostInfo interface {
	GetHostInfo() (HostInfoData, error)
	GetCloudHostType() (string, error)
}

type HostInfoCommon struct {
	cloudMonitoring bool
	agentVersion    string
	cloud.Harvester
}

var _ HostInfo = (*HostInfoCommon)(nil)

type HostInfoData struct {
	System   string `json:"id"`
	HostType string `json:"host_type"`
	CpuName  string `json:"cpu_name"`
	// Number of cores within a single CPU ('cpu cores' field in /proc/cpuinfo)
	// It is shown as 'coreCount' in New Relic UI
	CpuNum string `json:"cpu_num"`
	// Total number of cores in all the CPUs
	// It is shown as 'processorCount' in New Relic UI
	TotalCpu        string `json:"total_cpu"`
	Ram             string `json:"ram"`
	UpSince         string `json:"boot_timestamp"`
	AgentVersion    string `json:"agent_version"`
	AgentName       string `json:"agent_name"`
	OperatingSystem string `json:"operating_system"`

	// cloud metadata
	CloudData `mapstructure:",squash"`
}

// NewHostInfoCommon return a new HostInfoCommon structure that implements HostInfo.
func NewHostInfoCommon(agentVersion string, enableCloudMonitoring bool, cloudHarvester cloud.Harvester) *HostInfoCommon {
	return &HostInfoCommon{
		enableCloudMonitoring,
		agentVersion,
		cloudHarvester,
	}
}

// GetHostInfo returns the common host information data agnostic to the OS.
func (h *HostInfoCommon) GetHostInfo() (HostInfoData, error) {
	var cloudInfo CloudData
	var err error

	if h.cloudMonitoring {
		cloudInfo, err = getCloudData(h)
		if err != nil {
			return HostInfoData{}, err
		}
	}

	return HostInfoData{
		System:       "system",
		AgentName:    "Infrastructure",
		AgentVersion: h.agentVersion,
		CloudData:    cloudInfo,
	}, nil
}

// GetCloudHostType returns the cloud host type if available, "unknown" if not.
func (h *HostInfoCommon) GetCloudHostType() (string, error) {
	hostType := "unknown"

	if !h.cloudMonitoring ||
		h.GetCloudType() == cloud.TypeNoCloud ||
		h.GetCloudType() == cloud.TypeInProgress {
		return hostType, ErrNoCloudHostTypeNotAvailable
	}

	response, err := h.GetHostType()
	if err != nil {
		return hostType, err
	}

	return response, err
}

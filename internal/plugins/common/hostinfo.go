// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package common

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
	AwsCloudData     `mapstructure:",squash"`
	AzureCloudData   `mapstructure:",squash"`
	GoogleCloudData  `mapstructure:",squash"`
	AlibabaCloudData `mapstructure:",squash"`
}

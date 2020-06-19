// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows
// +build amd64

package windows

import (
	"fmt"
	"sort"
	"testing"

	"github.com/StackExchange/wmi"
	"github.com/stretchr/testify/assert"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
)

// See https://msdn.microsoft.com/en-us/library/aa394418(v=vs.85).aspx
type Win32_Service struct {
	Name        string
	DisplayName string
	State       string
	ProcessId   uint32
}

func getDatasetWMI() (result agent.PluginInventoryDataset, err error) {
	var wmiResults []Win32_Service

	// Only get running services which are set to start automatically.
	// "Noisy" services which the OS starts/stops based on various events generally have a StartMode of Manual, so we want to ignore those.
	wmiQuery := wmi.CreateQuery(&wmiResults, `WHERE State = "Running" AND StartMode = "Auto"`)
	if err = wmi.QueryNamespace(wmiQuery, &wmiResults, config.DefaultWMINamespace); err != nil {
		return result, fmt.Errorf("Error querying WMI: %s", err)
	}

	for _, wmiResult := range wmiResults {
		result = append(result, Output_Win32_Service{
			Name:        wmiResult.Name,
			DisplayName: wmiResult.DisplayName,
			State:       wmiResult.State,
			ProcessId:   fmt.Sprintf("%v", wmiResult.ProcessId),
		})
	}
	return
}

func TestServicesWin32AndWMI(t *testing.T) {
	plugin := ServicesPlugin{}

	services, err := plugin.getDataset()
	assert.NoError(t, err)

	servicesWMI, err := getDatasetWMI()
	assert.NoError(t, err)

	// There is a special case with the service LSM (Local Service Manager) that makes WMI fail to get the information
	// without admin privileges (Win32 does indeed return that information with user privileges).
	// This happens at least in Windows 10 x64. So we remove it from the data to avoid this outlier issue.
	for i := range services {
		if services[i].SortKey() == "LSM" {
			services = append(services[:i], services[i+1:]...)
			break
		}
	}
	for i := range servicesWMI {
		if servicesWMI[i].SortKey() == "LSM" {
			servicesWMI = append(servicesWMI[:i], servicesWMI[i+1:]...)
			break
		}
	}

	assert.Equal(t, services.Len(), servicesWMI.Len())
	sort.Sort(services)
	sort.Sort(servicesWMI)
	for i := 0; i < services.Len(); i++ {
		assert.Equal(t, services[i], servicesWMI[i])
	}
}

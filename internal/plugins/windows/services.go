// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build windows

package windows

import (
	"fmt"
	"time"

	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

type ServicesPlugin struct {
	agent.PluginCommon
	frequency time.Duration
}

// All output must be strings, but WMI requires types to match its internal data
// formats so we have to do a translation struct with all strings.
type Output_Win32_Service struct {
	Name        string `json:"id"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
	ProcessId   string `json:"pid"`
}

// Service Startup codes
const (
	ServiceBoot = iota
	ServiceSystem
	ServiceAutomatic
	ServiceManual
	ServiceDisabled
)

// Service state descriptions
const (
	ServiceStateContinuePending = "ContinuePending"
	ServiceStatePausePending    = "PausePending"
	ServiceStatePaused          = "Paused"
	ServiceStateRunning         = "Running"
	ServiceStateStartPending    = "StartPending"
	ServiceStateStopPending     = "StopPending"
	ServiceStateStopped         = "Stopped"
)

func (self Output_Win32_Service) SortKey() string {
	return self.Name
}

var slog = log.WithComponent("servicesPlugin")

func NewServicesPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	return &ServicesPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.WindowsServicesRefreshSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_WINDOWS_SERVICES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self *ServicesPlugin) getServicePID(mgr windows.Handle, serviceName string) (pid uint32, serviceState uint32, err error) {
	serviceNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return
	}

	service, err := windows.OpenService(mgr, serviceNamePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return
	}
	defer windows.CloseServiceHandle(service)

	status := windows.SERVICE_STATUS_PROCESS{}
	var bytesNeeded uint32
	err = windows.QueryServiceStatusEx(service, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafe.Pointer(&status)), uint32(unsafe.Sizeof(status)), &bytesNeeded)
	if err != nil {
		return
	}

	return status.ProcessId, status.CurrentState, nil
}

func (self *ServicesPlugin) getDataset() (result agent.PluginInventoryDataset, err error) {
	// Windows registry path that contains all the services on the local machine
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `System\CurrentControlSet\Services\`, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return result, fmt.Errorf("Error opening services key: %s", err)
	}
	defer key.Close()

	// Open the service control manager to read the active database on the local machine
	mgr, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return result, fmt.Errorf("Error opening service control manager: %s", err)
	}
	defer windows.CloseServiceHandle(mgr)

	services, err := key.ReadSubKeyNames(0)
	for _, serviceName := range services {
		serviceKey, err := registry.OpenKey(key, serviceName, registry.QUERY_VALUE)
		if err != nil {
			return result, fmt.Errorf("Error opening key %s: %s", serviceName, err)
		}

		// This value determines the startup type of the service: 0 = Boot, 1 = System, 2 = Automatic, 3 = Manual, 4 = Disabled)
		val, _, err := serviceKey.GetIntegerValue("Start")
		if err != nil {
			// This service is not valid
			serviceKey.Close()
			continue
		}

		// Only get running services which are set to start automatically.
		// "Noisy" services which the OS starts/stops based on various events generally have a StartMode of Manual, so we want to ignore those.
		if val == ServiceAutomatic {
			pid, state, err := self.getServicePID(mgr, serviceName)
			// check the service is currently running
			if pid == 0 || state != windows.SERVICE_RUNNING {
				serviceKey.Close()
				if err != nil {
					slog.WithError(err).WithField("service", serviceName).Debug("Error getting service PID")
				}
				continue
			}

			// https://docs.microsoft.com/en-us/windows/desktop/intl/locating-redirected-strings
			displayName, err := serviceKey.GetMUIStringValue("DisplayName")
			if err != nil {
				displayName, _, err = serviceKey.GetStringValue("DisplayName")
				if err != nil {
					serviceKey.Close()
					slog.WithError(err).WithField("service", serviceName).Debug("Error getting service %s DisplayName.")
					continue
				}
			}

			result = append(result, Output_Win32_Service{
				Name:        serviceName,
				DisplayName: displayName,
				State:       ServiceStateRunning,
				ProcessId:   fmt.Sprintf("%v", pid),
			})
		}

		serviceKey.Close()
	}

	return
}

func (self *ServicesPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		slog.Debug("Disabled.")
		return
	}

	// Introduce some jitter to wait randomly before reporting based on frequency time
	time.Sleep(config.JitterFrequency(self.frequency))

	refreshTimer := time.NewTicker(self.frequency)
	for {
		dataset, err := self.getDataset()
		if err != nil {
			slog.WithError(err).Error("services plugin can't get dataset")
		}
		self.EmitInventory(dataset, self.Context.AgentIdentifier())
		<-refreshTimer.C
	}
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// The initialize package performs OS-specific initialization actions during the
// startup of the agent. The execution order of the functions in this package is:
// 1 - OsProcess (when the operating system process starts and the configuration is loaded)
// 2 - AgentService (before the Agent starts)
package initialize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/StackExchange/wmi"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const (
	aboveNormalPriorityClass = 0x00008000
	belowNormalPriorityClass = 0x00004000
	highPriorityClass        = 0x00000080
	idlePriorityClass        = 0x00000040
	normalPriorityClass      = 0x00000020
	realtimePriorityClass    = 0x00000100
	agentTemporaryFolder     = "C:\\ProgramData\\New Relic\\newrelic-infra\\tmp"
	errInvalidFunction       = syscall.Errno(1)
)

var priorityClasses = map[string]uint{
	"Normal":      normalPriorityClass,
	"Idle":        idlePriorityClass,
	"High":        highPriorityClass,
	"RealTime":    realtimePriorityClass,
	"BelowNormal": belowNormalPriorityClass,
	"AboveNormal": aboveNormalPriorityClass,
}

// AgentService performs OS-specific initialization steps for the Agent service.
// It is executed after the initialize.osProcess function
func AgentService(cfg *config.Config) error {
	removeFunc = removeAllWithFallback

	err := emptyTemporaryFolder(cfg)
	if err != nil {
		log.WithField("temporaryFolder", agentTemporaryFolder).
			WithError(err).
			Error("error emptying temporary folder")
	}

	logger := log.WithField("action", "AgentService")
	// Set up Windows shared WMI Interface if active
	if !cfg.DisableWinSharedWMI {
		s, werr := wmi.InitializeSWbemServices(wmi.DefaultClient)
		if werr != nil {
			logger.WithError(werr).Error("Could not start Windows Shared WMI Interface")
			return werr
		}
		wmi.DefaultClient.SWbemServicesClient = s
		logger.Debug("Enabled Windows Shared WMI Interface.")
	} else {
		logger.Debug("Disabled Windows Shared WMI Interface.")
	}

	return nil
}

// removeAllWithFallback behaves like os.RemoveAll, but falls back to a manual
// recursive delete if the modern delete was rejected with ERROR_INVALID_FUNCTION.
func removeAllWithFallback(path string) error {
	err := os.RemoveAll(path)
	if err == nil {
		return nil
	}

	if !errors.Is(err, errInvalidFunction) {
		return fmt.Errorf("failed to remove path: %w", err)
	}

	return removeAllClassic(path)
}

func removeAllClassic(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			err = removeAllClassic(child)
			if err != nil {
				return err
			}

			continue
		}

		err = os.Remove(child)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove path: %w", err)
		}
	}

	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove path: %w", err)
	}

	return nil
}

// OsProcess performs initialization steps that are exclusive to the target OS
func OsProcess(config *config.Config) error {
	if config.WinProcessPriorityClass != "" {
		log.Info("Setting newrelic-infra process priority class to ", config.WinProcessPriorityClass)

		if priorityClass, ok := priorityClasses[config.WinProcessPriorityClass]; !ok {
			log.WithFields(logrus.Fields{
				"action":        "OsProcess",
				"providedValue": config.WinProcessPriorityClass,
			}).Warn("Invalid priority class. Valid values are: " +
				"Normal, Idle, High, RealTime, BelowNormal, AboveNormal. Running with default priority.")
		} else {
			err := setPriorityClass(priorityClass)
			if err != nil {
				log.WithFields(logrus.Fields{
					"action":        "OsProcess",
					"providedValue": config.WinProcessPriorityClass,
				}).WithError(err).Warn("Can't set newrelic-infra priority to %q. Running with default priority.")
			}
		}
	}
	return nil
}

func setPriorityClass(priorityClass uint) error {
	modKernel32 := syscall.NewLazyDLL("kernel32.dll")
	if modKernel32 == nil {
		return errors.New("can't load kernel32.dll")
	}
	procSetPriorityClass := modKernel32.NewProc("SetPriorityClass")
	if procSetPriorityClass == nil {
		return errors.New("can't load 'SetPriorityClass' process from kernel32.dll")
	}

	currentProcess, err := syscall.GetCurrentProcess()
	if err != nil {
		return err
	}

	r1, _, err := procSetPriorityClass.Call(uintptr(currentProcess), uintptr(priorityClass))

	if r1 == 0 {
		return fmt.Errorf("can't set priority class to %d: %s", priorityClass, err)
	}
	return nil
}

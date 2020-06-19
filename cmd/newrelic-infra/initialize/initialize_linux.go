// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package initialize performs OS-specific initialization actions during the
// startup of the agent. The execution order of the functions in this package is:
// 1 - OsProcess (when the operating system process starts and the configuration is loaded)
// 2 - AgentService (before the Agent starts)
package initialize

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/disk"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// AgentService performs OS-specific initialization steps for the Agent service.
// It is executed after the initialize.osProcess function.
func AgentService(cfg *config.Config) error {
	return nil
}

// OsProcess performs initialization steps for the OS process that contains the
// agent. It is executed before the initialize.AgentService function.
func OsProcess(config *config.Config) error {
	// Only check pidfile if the environment variable is not set and the agent
	// is not running in a container.
	if os.Getenv("PIDFILE") == "" && !config.IsContainerized {
		if err := verifySingularity(config.PidFile); err != nil {
			log.WithField("pidFile", config.PidFile).
				WithError(err).
				Error("Can't verify if agent is running as singleton.")
			os.Exit(1)
			return err
		}
	} else {
		log.Debugf("Not managing pid-file.")
	}

	// Check if the SDK temp folder, if it exists, belongs to the user running
	// the agent. Otherwise, create it.
	// This could be abused for malicious purposes if the user running the agent
	// is not the owner.
	err := assertTempFolderOwnership(config)

	return err
}

func verifySingularity(pidfile string) error {
	// Check if the file and containing folders do not exist, and try to create
	// them. If the user is running as non-root, they get a meaningful error.
	// This code acts as fallback in case /var/run/newrelic-infra is cleared up.
	pidFolder := path.Dir(pidfile)
	_, err := os.Stat(pidFolder)
	if os.IsNotExist(err) {
		// If the user removed the pid file folder, recreate it.
		// This only works when running the agent as root.
		if err := os.MkdirAll(pidFolder, 0755); err != nil {
			return fmt.Errorf("No pid-file. Can't create pid-file folder, err: %s", err.Error())
		}
	}

	pidBytes, err := ioutil.ReadFile(pidfile)
	if err != nil {
		// If the file does not exist, this is fine.
		if pErr, ok := err.(*os.PathError); !ok || !os.IsNotExist(pErr.Err) {
			return err
		}
	}
	if len(pidBytes) > 0 {
		pidString := strings.Trim(string(pidBytes), " \n")
		// Verify that /proc/{pid} does not exist (the process is gone).
		_, err := os.Stat(helpers.HostProc(pidString))
		if pathError, ok := err.(*os.PathError); !ok || !os.IsNotExist(pathError.Err) {
			return fmt.Errorf("pid-file already exists. Can't guarantee that no other newrelic-infra agent is running.")
		}
	}

	if err := disk.WriteFile(pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("Can't write pid-file: %s", err)
	}
	return nil
}

func assertTempFolderOwnership(config *config.Config) error {

	// Folder already exists. Check ownership.
	if fi, err := os.Stat(config.DefaultIntegrationsTempDir); !os.IsNotExist(err) {
		var currentUserUID string
		currentUser, err := user.Current()
		if err != nil {
			log.WithError(err).Warn("Failed to get current user. Assuming 'root (0)'.")
			currentUserUID = "0"
		} else {
			currentUserUID = currentUser.Uid
		}
		UID, _ := strconv.Atoi(currentUserUID)

		// Get UID and GID of a file. Solution inspired by https://stackoverflow.com/q/58179647
		var folderUID int
		if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
			folderUID = int(stat.Uid)
		}

		logEntry := log.WithField("user_id", UID).
			WithField("folder_uid", folderUID).
			WithField("folder", config.DefaultIntegrationsTempDir)

		if folderUID == UID {
			logEntry.
				WithField("folder", config.DefaultIntegrationsTempDir).
				Debug("Temp folder belongs to user running the agent. Continuing.")
			return nil
		}

		logEntry.Warn("Temp folder belongs to a different user. Trying to recreate folder for security purposes...")
		err = os.RemoveAll(config.DefaultIntegrationsTempDir)
		if err != nil {
			errLog := log.WithField("folder", config.DefaultIntegrationsTempDir).
				WithError(err)
			errLog.Warn("Failed to remove temp folder. Cannot continue until the folder is removed.")

			return err
		}
	}

	// If we got here, either the folder does not exist or we removed it, so let's create it.
	err := os.MkdirAll(config.DefaultIntegrationsTempDir, 0755)
	if err != nil {
		log.WithField("folder", config.DefaultIntegrationsTempDir).
			WithError(err).
			Warn("Failed to create temp folder. This may cause issues when executing integrations.")
	}

	return nil
}

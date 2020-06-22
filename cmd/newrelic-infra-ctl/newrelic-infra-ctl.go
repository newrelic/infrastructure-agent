// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:generate goversioninfo

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/newrelic/infrastructure-agent/pkg/ipc"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/ctl/sender"
	"github.com/sirupsen/logrus"
)

var (
	agentPID    int
	containerID string
	apiVersion  string
)

func init() {
	flag.IntVar(
		&agentPID,
		"pid",
		0,
		"New Relic infrastructure agent PID",
	)

	flag.StringVar(
		&containerID,
		"cid",
		"",
		"New Relic infrastructure agent container ID (Containerised agent)",
	)

	flag.StringVar(
		&apiVersion,
		"docker-api-version",
		config.DefaultDockerApiVersion,
		"Docker API version [Optional] (Containerised agent)",
	)
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	// Enables Control+C termination
	go func() {
		s := make(chan os.Signal, 1)
		signal.Notify(s, syscall.SIGQUIT)
		<-s
		cancel()
	}()

	client, err := getClient()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize the notification client.")
	}

	// Default message is "enable verbose logging" to maintain backwards compatibility.
	msg := ipc.EnableVerboseLogging
	logrus.Debug("Sending message to agent: " + fmt.Sprint(msg))
	if err := client.Notify(ctx, msg); err != nil {
		logrus.WithError(err).Fatal("Error occurred while notifying the NRI Agent.")
	}

	logrus.Infof("Notification successfully sent to the NRI Agent with ID '%s'", client.GetID())
}

// getClient returns an agent notification client.
func getClient() (sender.Client, error) {
	if runtime.GOOS == "windows" || agentPID != 0 {
		return sender.NewClient(agentPID)
	}
	if containerID != "" {
		return sender.NewContainerisedClient(apiVersion, containerID)
	}
	return sender.NewAutoDetectedClient(apiVersion)
}

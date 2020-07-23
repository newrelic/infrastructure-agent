// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package sender

import (
	"context"

	"github.com/newrelic/infrastructure-agent/pkg/helpers/windows"
	"github.com/newrelic/infrastructure-agent/pkg/ipc"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

var wlog = log.WithComponent("Windows Client")

// windows named pipe based agent control client.
type windowsClient struct {
	pipe string
}

// NewClient creates an agent controller client for windows.
// Windows doesn't require the Pid parameter
func NewClient(pid int) (c Client, err error) {
	return NewClientWithName(pid, windows.GetPipeName("newrelic-infra"))
}

// NewClientWithName creates an agent controller client for windows with a configurable pipe name
// Windows doesn't require the Pid parameter
func NewClientWithName(_ int, pipeName string) (c Client, err error) {
	c = &windowsClient{pipe: pipeName}
	return
}

// Notify will notify a running agent process by sending a message to its message handler.
func (c *windowsClient) Notify(_ context.Context, message ipc.Message) error {
	wlog.Debug("Notify requested. sending message to pipe...")
	err := windows.PostNotificationMessage(c.pipe, message)
	if err != nil {
		wlog.WithError(err).Error("failed to write message to pipe. make sure the Agent is running")
		return err
	}
	return nil
}

// Return the identification for the notified agent.
func (c *windowsClient) GetID() string {
	return "windows"
}

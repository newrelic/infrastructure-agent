// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux
// +build harvest

package harvest

import (
	"os/exec"
	"os/user"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	pluginsLinux "github.com/newrelic/infrastructure-agent/internal/plugins/linux"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFacter(t *testing.T) {
	const agentIdentifier = "mock agent id"

	if _, err := exec.LookPath("facter"); err != nil {
		t.Skip("Test can only be run in systems with facter installed")
	}

	ctx := new(mocks.AgentContext)
	usr, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	ctx.On("Config").Return(&config.Config{
		FacterIntervalSec: 1,
		FacterHomeDir:     usr.HomeDir,
	})
	ctx.On("AgentIdentifier").Return(agentIdentifier)
	ch := make(chan agent.PluginOutput)
	ctx.On("SendData", mock.Anything).Return().Run(func(args mock.Arguments) {
		ch <- args[0].(agent.PluginOutput)
	})

	facterPlugin := pluginsLinux.NewFacterPlugin(ctx)
	go facterPlugin.Run()

	actual := <-ch
	ctx.AssertExpectations(t)
	assert.True(t, len(actual.Data) > 0)

	facts := map[string]struct{}{
		"kernel":       {},
		"timezone":     {},
		"id":           {},
		"gid":          {},
		"architecture": {},
	}

	for _, actualFact := range actual.Data {
		fact := actualFact.(pluginsLinux.FacterItem)
		if _, ok := facts[fact.Name]; ok {
			return // at least one item is found. We assume facter works
		}
	}

	assert.Failf(t, "it seems no facter items have been found", "in: %#v", actual.Data)
}

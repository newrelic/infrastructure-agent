// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"io/ioutil"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/pkg/errors"
)

type SysctlSubscriberPlugin struct {
	SysctlPlugin
	watcher *fsnotify.Watcher
}

// NewSysctlSubscriberMonitor creates a /proc/sys parser, walking once the full path and then subscribing to
// changed FS events.
func NewSysctlSubscriberMonitor(id ids.PluginID, ctx agent.AgentContext) (*SysctlSubscriberPlugin, error) {
	sysPoller := NewSysctlPollingMonitor(id, ctx)

	if sysPoller.frequency <= config.FREQ_DISABLE_SAMPLING {
		return nil, PluginDisabledErr
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "cannot create sys watcher")
	}

	err = watcher.Add(sysPoller.procSysDir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot watch on sys filesystem")
	}

	return &SysctlSubscriberPlugin{
		SysctlPlugin: *sysPoller,
		watcher:      watcher,
	}, nil
}

// Run is where you implement your plugin logic
func (p *SysctlSubscriberPlugin) Run() {
	ticker := time.NewTicker(1)

	var initialSubmitOk bool
	var needsFlush bool
	var deltas agent.PluginInventoryDataset
	for {
		select {
		case <-ticker.C:
			if !initialSubmitOk {
				initialDataset, err := p.Sysctls()
				if err != nil {
					sclog.WithError(err).Error("fetching sysctl initial data")
				} else {
					p.EmitInventory(initialDataset, p.Context.AgentIdentifier())
					initialSubmitOk = true
				}
			} else if needsFlush {
				p.EmitInventory(deltas, p.Context.AgentIdentifier())
				needsFlush = false
				deltas = agent.PluginInventoryDataset{}
			}
			ticker.Stop()
			ticker = time.NewTicker(p.frequency)

		case event, ok := <-p.watcher.Events:
			if !ok {
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				needsFlush = true
				output, err := ioutil.ReadFile(event.Name)
				if err != nil {
					sclog.WithField("file", event.Name).Debug("Cannot read sys file.")
				} else {
					deltas = append(deltas, p.newSysctlItem(event.Name, output))
				}
			}
		}
	}
}

// deprecated, just for testing purposes
func (p *SysctlSubscriberPlugin) EventsCh() chan fsnotify.Event {
	return p.watcher.Events
}

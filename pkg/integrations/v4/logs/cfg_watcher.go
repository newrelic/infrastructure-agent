// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	ctx2 "context"
	"github.com/fsnotify/fsnotify"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/fs"
	"github.com/newrelic/infrastructure-agent/pkg/log"
)

// ConfigChangesWatcher will look in a path for changes in the configuration.
type ConfigChangesWatcher struct {
	watcher *fsnotify.Watcher
	logger  log.Entry
	path    string
}

// NewConfigChangesWatcher creates a new instance of ConfigChangesWatcher.
func NewConfigChangesWatcher(path string) *ConfigChangesWatcher {
	logger := log.WithComponent("integrations.Supervisor").WithField("process", "config-changes-watcher")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.WithError(err).Warn("Cannot enable configuration automatic reloading for log-forwarder")
	}
	return &ConfigChangesWatcher{
		watcher: watcher,
		logger:  logger,
		path:    path,
	}
}

// Watch is registering a channel to push notifications when a change in the watched path is detected.
func (ccw *ConfigChangesWatcher) Watch(ctx ctx2.Context, changes chan<- struct{}) {
	if ccw.watcher == nil {
		return
	}
	ccw.logger.Debugf("adding path to watching %v", ccw.path)
	if err := ccw.watcher.Watch(ccw.path); err != nil {
		ccw.logger.WithError(err).Warn("cant watch for file changes in folder")
		return
	}

	go ccw.watchForChanges(ctx, changes)
}

// watch for changes in the plugins directories and loads/cancels/reloads the affected integrations
func (ccw *ConfigChangesWatcher) watchForChanges(ctx ctx2.Context, changes chan<- struct{}) {

	if ccw.watcher == nil {
		return
	}

	ccw.logger.Debug("Watching for logging config file changes.")
	for {
		select {
		case event := <-ccw.watcher.Event:
			ccw.handleFileEvent(event, changes)
		case err := <-ccw.watcher.Error:
			ccw.logger.WithError(err).Debug("Error occurred while watching for logging config file changes.")
		case <-ctx.Done():
			ccw.logger.Debug("Stopping logging config changes watcher.")
			if err := ccw.watcher.Close(); err != nil {
				ccw.logger.WithError(err).Debug("Error occurred while stopping watcher for logging config file changes.")
			}
			return
		}
	}
}

func (ccw *ConfigChangesWatcher) handleFileEvent(event *fsnotify.FileEvent, signalReload chan<- struct{}) {
	helog := ccw.logger.WithField("function", "handleFileEvent")

	if event == nil {
		helog.Debug("Unexpected nil watcher event. Ignoring.")
		return
	}
	elog := helog.
		WithField("event", event.String()).
		WithField("file_name", event.Name)
	elog.Debug("Received File event.")

	isDelete := event.IsDelete() || event.IsRename()
	isCreate := event.IsCreate()
	isWrite := isCreate || event.IsModify()
	if !isDelete && !isWrite {
		elog.Debug("Ignoring File event.")
		return
	}

	if err := fs.ValidateYAMLFile(event.Name, isDelete); err != nil {
		elog.WithField("file", event.Name).WithError(err).
			Debug("Not an existing YAML file. Ignoring.")
		return
	}

	select {
	case signalReload <- struct{}{}:
	default:
	}
}

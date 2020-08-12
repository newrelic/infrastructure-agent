// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"

	"github.com/newrelic/infrastructure-agent/internal/agent"
)

const (
	EXTERNAL_DIR = "external.d"
)

var monitoredFiles map[string]bool
var movedFiles map[string]bool

type ExternalDFile struct {
	Files []struct {
		Path string `json:"path"`
	} `json:"files"`
}

type ConfigFilePlugin struct {
	agent.PluginCommon
	externalDDir  string
	watcher       *fsnotify.Watcher
	flushInterval time.Duration
	logger        log.Entry
}

func NewConfigFilePlugin(id ids.PluginID, ctx agent.AgentContext) (plugin *ConfigFilePlugin) {
	movedFiles = make(map[string]bool, 0)
	watcher, err := fsnotify.NewWatcher()
	logger := slog.WithPlugin(id.String())
	if err != nil {
		logger.WithError(err).Error("can't instantiate file watcher")
	}
	return &ConfigFilePlugin{
		agent.PluginCommon{ID: id, Context: ctx},
		path.Join(ctx.Config().AgentDir, EXTERNAL_DIR),
		watcher,
		time.Second * 15,
		logger,
	}
}

func (self *ConfigFilePlugin) WithFlushInterval(i time.Duration) *ConfigFilePlugin {
	self.flushInterval = i
	return self
}

func parseExternalDFile(p string) (files []string, err error) {
	var (
		buf  []byte
		conf ExternalDFile
	)
	buf, err = ioutil.ReadFile(p)
	if err != nil {
		return
	}
	err = json.Unmarshal(buf, &conf)
	if err != nil {
		return
	}
	for _, file := range conf.Files {
		if file.Path == "" {
			slog.WithField(
				"path", fmt.Sprintf("files.d/%s", p),
			).Warn("Empty path attribute on an urlEntry")
		} else if !filepath.IsAbs(file.Path) {
			slog.WithFields(logrus.Fields{
				"path":            fmt.Sprintf("files.d/%s", p),
				"nonAbsolutePath": file.Path,
			}).Warn("Ignoring non-absolute path")
		} else {
			// all good!
			files = append(files, file.Path)
		}
	}
	return
}

func parseExternalD(dir string) (configFiles map[string]bool, err error) {
	configFiles = make(map[string]bool, 0)
	err = filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) (err error) {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				slog.WithField("path", p).Warn("Ignoring non-existing path")
				return nil
			}
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		files, err := parseExternalDFile(p)
		if err != nil {
			slog.WithField("path", p).WithError(err).Error("Invalid external config")
			// continue processing
			err = nil
			return
		}

		for _, f := range files {
			configFiles[f] = true
		}
		return
	})
	return
}

func fileTypeString(fi os.FileInfo) (fileType string) {
	mode := fi.Mode()

	switch {
	case mode.IsRegular():
		fileType = "regular file"
	case mode.IsDir():
		fileType = "directory"
	case mode&os.ModeSymlink == os.ModeSymlink:
		fileType = "symlink"
	case mode&os.ModeDevice == os.ModeDevice:
		fileType = "device"
	case mode&os.ModeNamedPipe == os.ModeNamedPipe:
		fileType = "named pipe"
	case mode&os.ModeSocket == os.ModeSocket:
		fileType = "socket"
	default:
		fileType = ""
	}

	return fileType
}

func getPluginDataset() (dataset agent.PluginInventoryDataset, err error) {
	for filename := range monitoredFiles {
		var d FileData
		if d, err = getFileData(filename); err != nil {
			// return the error if it's anything other than the file not existing
			if !os.IsNotExist(err) {
				// if the file was simply not found, ignore the error, means it's just gone
				slog.WithError(err).WithField(
					"file", filename,
				).Error("error collecting data for file")
			}
			err = nil
			continue
		}
		dataset = append(dataset, d)
	}
	return
}

var logFilePattern = regexp.MustCompile(`(.+?\.log$|/syslog$|/messages$|\/log\/|\bmotd$)`)

func isLogFile(fp string) (result bool) {
	return logFilePattern.Match([]byte(fp))
}

func shouldBeIgnored(fp string) (result bool) {
	if isLogFile(fp) {
		return true
	}

	fileInfo, err := os.Stat(fp)
	if err != nil {
		if os.IsNotExist(err) {
			result = true
		} else {
			// Unknown error looking up the file - let the monitor try to watch it and see what happens.
		}
	} else if fileInfo.IsDir() {
		result = true
	}
	return
}

func (self *ConfigFilePlugin) parseAndAddPaths() {
	var err error

	if monitoredFiles, err = parseExternalD(filepath.Join(self.Context.Config().AgentDir, EXTERNAL_DIR)); err != nil {
		self.logger.WithError(err).WithField(
			"configurations", EXTERNAL_DIR,
		).Error("Could not read configuration for files to watch")
	}

	ignored := 0
	for file := range monitoredFiles {
		if shouldBeIgnored(file) {
			ignored += 1
		} else {
			if err := self.watcher.Add(file); err != nil {
				self.logger.WithError(err).WithField("file", file).Error("Unable to add watch to file")
			}
		}
	}
	if ignored > 0 {
		self.logger.WithField(
			"ignoredFilesCount", ignored,
		).Info("ignoring files from external.d because they look like they're logfiles or similar")
	}
}

func (self *ConfigFilePlugin) Run() {

	// Start a ticker to check for external.d.
	// Basically, wen want to check external.d RIGHT AWAY, and if it doesn't exist,
	// start checking for its existence at 1 minute intervals. We also want to collect
	// the first dataset asap, but not before we've had a chance to parse external.d.
	// After the first cycle, we want to switch to self.flushInterval.
	checkTicker := time.NewTicker(1)
	flushTimer := time.NewTicker(1 * time.Second)

	externalDExists := false
	flushNeeded := false

	for {
		select {
		case <-checkTicker.C:
			// stop the ticker regardless, it gets recreated below if needed
			checkTicker.Stop()
			// if we successfully add a watch for external.d kill the ticker
			if err := self.watcher.Add(self.externalDDir); err == nil {
				externalDExists = true
				self.parseAndAddPaths()
				// since we just updated ALL THE FILES, we probably want to flush
				flushNeeded = true
			} else {
				// we need to keep checking
				checkTicker = time.NewTicker(1 * time.Minute)
			}

		case event := <-self.watcher.Events:
			if filepath.Dir(event.Name) == self.externalDDir {
				self.parseAndAddPaths()
				flushNeeded = true
			}

			if event.Op&fsnotify.Rename == fsnotify.Rename {
				movedFiles[event.Name] = true
			}

			if event.Op&fsnotify.Remove == fsnotify.Remove {
				flushNeeded = true
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				flushNeeded = true
			}

		case err := <-self.watcher.Errors:
			self.logger.WithError(err).Error("watcher received an error")

		case <-flushTimer.C:
			if !externalDExists {
				self.Unregister()
				continue
			}
			if flushNeeded {
				flushTimer.Stop()
				flushTimer = time.NewTicker(self.flushInterval)
				dataset, err := getPluginDataset()
				if err != nil {
					self.logger.WithError(err).Error("Fetching external data set")
				}
				self.EmitInventory(dataset, self.Context.AgentIdentifier())
				flushNeeded = false

				// re-add any files that may have been renamed in the last flush interval
				for file := range movedFiles {
					if err := self.watcher.Add(file); err != nil {
						self.logger.WithError(err).WithField(
							"file", file,
						).Error("Couldn't re-add file to watch")
					} else {
						delete(movedFiles, file)
					}
				}
			}
		}
	}
}

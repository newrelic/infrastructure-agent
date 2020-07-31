// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package delta

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PluginInfo holds information about agent plugins
type PluginInfo struct {
	Source         string           `json:"source"`
	Plugin         string           `json:"plugin"`
	FileName       string           `json:"filename"`
	LastSentID     int64            `json:"last_sent_id"` // Latest delta available on platform
	MostRecentIDs  map[string]int64 `json:"mru_ids"`      // Most recent ids per entity plugin delta (replaces obsolete "mru_id", as it does not support remote entities)
}

// newPluginInfo creates a new PluginInfo from plugin name and file
func newPluginInfo(name, fileName string) *PluginInfo {
	cleanFileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	return &PluginInfo{
		Source:         fmt.Sprintf("%s/%s", name, cleanFileName),
		Plugin:         name,
		FileName:       fileName,
		MostRecentIDs:  make(map[string]int64),
		LastSentID:     NO_DELTA_ID,
	}
}

// setDeltaID is used as backend-client reconciliation mechanism
func (pi *PluginInfo) setDeltaID(entityKey string, value int64) {
	pi.initialize()

	pi.MostRecentIDs[entityKey] = value
}

// increaseDeltaID triggered on plugin reap, prior to submission
func (pi *PluginInfo) increaseDeltaID(entityKey string) {
	pi.initialize()

	if v, ok := pi.MostRecentIDs[entityKey]; ok {
		pi.MostRecentIDs[entityKey] = v + 1
	} else {
		pi.MostRecentIDs[entityKey] = 1
	}
}

// deltaID provides delta ID for one of this plugin's entity
func (pi *PluginInfo) deltaID(entityKey string) int64 {
	pi.initialize()

	id, _ := pi.MostRecentIDs[entityKey]
	return id
}

func (pi *PluginInfo) initialize() {
	if pi.MostRecentIDs == nil {
		pi.MostRecentIDs = make(map[string]int64)
	}
}

// pluginSource2Info stores plugins info by source
type pluginSource2Info map[string]*PluginInfo

// ID returns plugin serialized ID.
func (pi *PluginInfo) ID() string {
	return pi.Source
}

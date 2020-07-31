// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package delta

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PluginInfo persisted information about plugins.
type PluginInfo struct {
	Source         string           `json:"source"`
	Plugin         string           `json:"plugin"`
	FileName       string           `json:"filename"`
	LastSentID     int64            `json:"last_sent_id"` // latest ID from platform, to decide whether archive or keep delta
	MostRecentIDs  map[string]int64 `json:"mru_ids"`      // latest IDs per entity plugin (replaces obsolete "mru_id", as it does not support remote entities)
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
func (p *PluginInfo) setDeltaID(entityKey string, value int64) {
	p.init()

	p.MostRecentIDs[entityKey] = value
}

// increaseDeltaID triggered on plugin reap, prior to submission
func (p *PluginInfo) increaseDeltaID(entityKey string) {
	p.init()

	if v, ok := p.MostRecentIDs[entityKey]; ok {
		p.MostRecentIDs[entityKey] = v + 1
	} else {
		p.MostRecentIDs[entityKey] = 1
	}
}

// deltaID provides delta ID for one of this plugin's entity
func (p *PluginInfo) deltaID(entityKey string) int64 {
	p.init()

	id, _ := p.MostRecentIDs[entityKey]
	return id
}

func (p *PluginInfo) init() {
	if p.MostRecentIDs == nil {
		p.MostRecentIDs = make(map[string]int64)
	}
}

// pluginSource2Info stores plugins info by source
type pluginSource2Info map[string]*PluginInfo

// ID returns plugin serialized ID.
func (p *PluginInfo) ID() string {
	return p.Source
}

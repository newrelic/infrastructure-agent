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
	Source   string              `json:"source"`
	Plugin   string              `json:"plugin"`
	FileName string              `json:"filename"`
	Entities map[string]PIEntity `json:"entities"`
}

// PIEntity persisted info about an entity for a plugin.
type PIEntity struct {
	MostRecentID int64 `json:"mru_id"`       // latest ID for an plugin entity to be used for submission
	LastSentID   int64 `json:"last_sent_id"` // latest ID from platform, decides whether archive or keep delta
}

// newPluginInfo creates a new PluginInfo from plugin name and file.
func newPluginInfo(name, fileName string) *PluginInfo {
	cleanFileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	return &PluginInfo{
		Source:   fmt.Sprintf("%s/%s", name, cleanFileName),
		Plugin:   name,
		FileName: fileName,
		Entities: make(map[string]PIEntity),
	}
}

// setLastSentID is used to store latest ID from platform.
func (p *PluginInfo) setLastSentID(entityKey string, value int64) {
	e := p.entity(entityKey)
	e.LastSentID = value
	p.Entities[entityKey] = e
}

// lastSentID retrieves last sent ID.
func (p *PluginInfo) lastSentID(entityKey string) int64 {
	return p.entity(entityKey).LastSentID
}

// setDeltaID is used as backend-client reconciliation mechanism.
func (p *PluginInfo) setDeltaID(entityKey string, value int64) {
	e := p.entity(entityKey)
	e.MostRecentID = value
	p.Entities[entityKey] = e
}

// increaseDeltaID triggered on plugin reap, prior to submission
func (p *PluginInfo) increaseDeltaID(entityKey string) {
	e := p.entity(entityKey)
	e.MostRecentID++
	p.Entities[entityKey] = e
}

// deltaID provides delta ID for one of this plugin's entity.
func (p *PluginInfo) deltaID(entityKey string) int64 {
	return p.entity(entityKey).MostRecentID
}

func (p *PluginInfo) entity(key string) PIEntity {
	if p.Entities == nil {
		p.Entities = make(map[string]PIEntity)
	}
	if _, ok := p.Entities[key]; !ok {
		p.Entities[key] = PIEntity{}
	}

	return p.Entities[key]
}

// pluginSource2Info stores plugins info by source.
type pluginSource2Info map[string]*PluginInfo

// ID returns plugin serialized ID.
func (p *PluginInfo) ID() string {
	return p.Source
}

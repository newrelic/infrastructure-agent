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
	Source         string `json:"source"`
	Plugin         string `json:"plugin"`
	FileName       string `json:"filename"`
	MostRecentID   int64  `json:"mru_id"`       // Most recent id assigned to a delta
	LastSentID     int64  `json:"last_sent_id"` // Most recent delta id sent to server
	FirstArchiveID int64  `json:"first_archive_id"`
}

// NewPluginInfo
func NewPluginInfo(dirName, fileName string) *PluginInfo {
	cleanFileName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	return &PluginInfo{
		Source:         fmt.Sprintf("%s/%s", dirName, cleanFileName),
		Plugin:         dirName,
		FileName:       fileName,
		MostRecentID:   NO_DELTA_ID,
		LastSentID:     NO_DELTA_ID,
		FirstArchiveID: NO_DELTA_ID,
	}
}

func (pi *PluginInfo) nextDeltaID() int64 {
	pi.MostRecentID = pi.MostRecentID + 1
	return pi.MostRecentID
}

// pluginSource2Info stores plugins info by source
type pluginSource2Info map[string]*PluginInfo

// ID returns plugin serialized ID.
func (pi *PluginInfo) ID() string {
	return pi.Source
}

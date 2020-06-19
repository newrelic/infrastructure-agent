// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package delta

// PluginInfo holds information about agent plugins
type PluginInfo struct {
	Source         string `json:"source"`
	Plugin         string `json:"plugin"`
	FileName       string `json:"filename"`
	MostRecentID   int64  `json:"mru_id"`       // Most recent id assigned to a delta
	LastSentID     int64  `json:"last_sent_id"` // Most recent delta id sent to server
	FirstArchiveID int64  `json:"first_archive_id"`
}

func (pi *PluginInfo) nextDeltaID() int64 {
	pi.MostRecentID = pi.MostRecentID + 1
	return pi.MostRecentID
}

type pluginIDMap map[string]*PluginInfo

// ID returns plugin serialized ID.
func (pi *PluginInfo) ID() string {
	return pi.Source
}

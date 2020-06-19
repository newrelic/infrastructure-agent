// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package inventoryapi

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/entity"

	"github.com/newrelic/infrastructure-agent/pkg/log"
)

const ResetAll = "all"

// Maximum size for the delta source field.
const MaxSourceLen = 100

type RawDelta struct {
	Source    string                 `json:"source"`
	ID        int64                  `json:"id"`
	Timestamp int64                  `json:"timestamp"`
	Diff      map[string]interface{} `json:"diff"`
	FullDiff  bool                   `json:"full_diff"` // See DiffType* constants
}

// validate is used to validate the RawDelta fields.
func (rd *RawDelta) validate() (bool, error) {
	if len(rd.Source) > MaxSourceLen {
		return false, fmt.Errorf("source field exceed %d chararcters limit, value: '%s', check integration name/prefix",
			MaxSourceLen, rd.Source)
	}
	return true, nil
}

// filterDeltas is used to remove invalid deltas.
func filterDeltas(deltas []*RawDelta) []*RawDelta {
	result := deltas[:0]
	for _, delta := range deltas {
		if valid, err := delta.validate(); !valid {
			log.WithField("action", "FilterDeltas").WithError(err).Error("inventory delta is invalid")
			continue
		}
		result = append(result, delta)
	}
	return result
}

// RawDeltaBlock groups RawDeltas to be processed in different blocks
type RawDeltaBlock []*RawDelta

type PostDeltaBody struct {
	EntityID entity.ID `json:"entityId,omitempty"`

	ExternalKeys []string `json:"entityKeys"`

	// Is this entity an agent's own host? Controls whether we display the entity as such and
	// track its connected status. Pointer allows nil for older agents which didn't send this field.
	IsAgent *bool `json:"isAgent"`

	Deltas []*RawDelta `json:"deltas"`
}

// Sortable implementation of a raw delta list so we can sort by ID
type DeltasByID []*RawDelta

func (a DeltasByID) Len() int           { return len(a) }
func (a DeltasByID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a DeltasByID) Less(i, j int) bool { return a[i].ID < a[j].ID }

type DeltaState struct {
	// NeedsReset was added Sept 2017 to fix the situation where the agent
	// sent id N and the server expected N+1 resulting in the ambiguous
	// SendNextID server response of N+1.  The agent only observes this in
	// versions > 1.0.783.
	NeedsReset   bool    `json:"needs_reset,omitempty"`
	LastStoredID int64   `json:"last_stored_id"` // Latest ID of what is stored, zero if none
	SendNextID   int64   `json:"send_next_id"`   // Starting ID that should be sent back, zero if send original
	Error        *string `json:"error,omitempty"`
}

type DeltaStateMap map[string]*DeltaState

// PostDeltaResponse is the response to a post delta request.
type PostDeltaResponse struct {
	Version  int64         `json:"version"`
	StateMap DeltaStateMap `json:"state_map"`
	Reset    string        `json:"reset,omitempty"` // Set to inventoryapi.ResetAll (all) to reset everything, blank means nothing
}

// BulkDeltaResponse is an entry in the bulk delta post, which is the
// PostDeltaResponse decorated with the request entityKeys and an error
// to indicate a failure.
type BulkDeltaResponse struct {
	PostDeltaResponse
	EntityKeys []string `json:"entityKeys"`
	Error      string   `json:"error,omitempty"`
}

func NewPostDeltaResponse() *PostDeltaResponse {
	dsm := &PostDeltaResponse{
		StateMap: make(DeltaStateMap),
	}
	return dsm
}

func NewRawDelta(source string, deltaID int64, timestamp int64, diff map[string]interface{}, full bool) *RawDelta {
	return &RawDelta{
		Source:    source,
		ID:        deltaID,
		Timestamp: timestamp,
		Diff:      diff,
		FullDiff:  full,
	}
}

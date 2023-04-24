// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

// Anything implementing the sortable interface must implement a
// method to return a string Sort key
type Sortable interface {
	SortKey() string
}

type PluginInventoryDataset []Sortable // PluginInventoryDataset is a slice of sortable things

// PluginInventoryDataset also implements the sort.Sort interface
func (pd PluginInventoryDataset) Len() int           { return len(pd) }
func (pd PluginInventoryDataset) Swap(i, j int)      { pd[i], pd[j] = pd[j], pd[i] }
func (pd PluginInventoryDataset) Less(i, j int) bool { return pd[i].SortKey() < pd[j].SortKey() }

// PluginOutput contains metadata about the inventory provided by Plugins, which will be used for its later addition
// to the delta store
type PluginOutput struct {
	Id            ids.PluginID
	Entity        entity.Entity
	Data          PluginInventoryDataset
	NotApplicable bool
}

func NewPluginOutput(id ids.PluginID, entity entity.Entity, data PluginInventoryDataset) PluginOutput {
	return PluginOutput{Id: id, Entity: entity, Data: data}
}

func NewNotApplicableOutput(id ids.PluginID) PluginOutput {
	return PluginOutput{Id: id, NotApplicable: true}
}

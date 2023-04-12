// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/internal/agent/types"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/sirupsen/logrus"
	"sort"
	"strings"
	"time"
)

const (
	defaultRemoveEntitiesPeriod = 48 * time.Hour
)

type PatchSender interface {
	Process() error
}

// Patcher performs the actions required to update the state of the inventory stored
// by the backend.
type Patcher interface {
	// Save will store the new data received from a plugin for future processing.
	Save(output types.PluginOutput) error

	// Reap will compare latest saved data with latest submitted and generate
	// deltas in storage if required.
	Reap()

	// Send will look for deltas in the storage and submit them to the backend.
	Send() error
}

type PatcherConfig struct {
	IgnoredPaths         map[string]struct{}
	AgentEntity          entity.Entity
	RemoveEntitiesPeriod time.Duration
}

// BasePatcher will keep the common functionality of a patcher.
type BasePatcher struct {
	deltaStore *delta.Store
	cfg        PatcherConfig
	lastClean  time.Time
}

func (b *BasePatcher) needsCleanup() bool {
	if b.lastClean.Equal(time.Time{}) {
		b.lastClean = time.Now()
		return false
	}

	removePeriod := b.cfg.RemoveEntitiesPeriod
	if removePeriod <= 0 {
		removePeriod = defaultRemoveEntitiesPeriod
	}

	needsCleanup := b.lastClean.Add(removePeriod).Before(time.Now())
	if needsCleanup {
		b.lastClean = time.Now()
	}
	return needsCleanup
}

// save will take a PluginOutput and persist it in the store.
func (b *BasePatcher) save(pluginOutput types.PluginOutput) error {

	if pluginOutput.Data == nil {
		pluginOutput.Data = make(types.PluginInventoryDataset, 0)
	}

	sort.Sort(pluginOutput.Data)

	simplifiedPluginData := make(map[string]interface{})

	for _, data := range pluginOutput.Data {
		if data == nil {
			continue
		}
		sortKey := data.SortKey()

		// Filter out ignored inventory data before writing the file out
		pluginSource := fmt.Sprintf("%s/%s", pluginOutput.Id, sortKey)
		if b.isIgnored(pluginSource) {
			continue
		}
		simplifiedPluginData[sortKey] = data
	}

	return b.deltaStore.SavePluginSource(
		pluginOutput.Entity.Key.String(),
		pluginOutput.Id.Category,
		pluginOutput.Id.Term,
		simplifiedPluginData,
	)
}

// isIgnored will check if a specific plugin output should be ignored according to the configuration.
func (b *BasePatcher) isIgnored(pluginSource string) bool {
	if b.cfg.IgnoredPaths == nil {
		return false
	}
	_, ignored := b.cfg.IgnoredPaths[strings.ToLower(pluginSource)]
	return ignored
}

// reapEntity will tell storage to generate deltas for a specific entity.
func (b *BasePatcher) reapEntity(entityKey entity.Key) {
	// Reap generates deltas from last iteration and persist.
	entityKeyStr := entityKey.String()
	err := b.deltaStore.UpdatePluginsInventoryCache(entityKeyStr)
	if err != nil {
		ilog.WithFields(logrus.Fields{
			"entityKey": entityKeyStr,
		}).WithError(err).Error("failed to update inventory cache")
		return
	}
}

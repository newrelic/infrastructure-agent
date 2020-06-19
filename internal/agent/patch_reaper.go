// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package agent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/internal/agent/delta"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"
)

// patchReaper gets the inventory data that has changed since the last reap and commits it into storage
type patchReaper struct {
	entityKey string
	store     *delta.Store
}

var prlog = log.WithComponent("PatchReaper")

func newPatchReaper(entityKey string, store *delta.Store) *patchReaper {
	if store == nil {
		prlog.WithField("entityKey", entityKey).Error("creating patch reaper: delta store can't be nil")
		panic("creating patch reaper: delta store can't be nil")
	}

	return &patchReaper{
		entityKey: entityKey,
		store:     store,
	}
}

// CleanupOldPlugins deletes old json from plugins that have been
// deprecated or are no longer used
func (p *patchReaper) CleanupOldPlugins(plugins []ids.PluginID) {
	for _, plugin := range plugins {
		// first check if file exists
		filename := filepath.Join(p.store.DataDir, plugin.Category, fmt.Sprintf("%s.json", plugin.Term))
		if _, err := os.Stat(filename); err != nil {
			continue
		}
		// next, remove the source file first, which will show
		// as a deleted delta
		if err := os.Remove(filename); err != nil {
			prlog.WithFields(logrus.Fields{
				"path":      filename,
				"entityKey": p.entityKey,
			}).WithError(err).Error("failed to delete plugin source file")
		}
	}
}

func (p *patchReaper) Reap() {
	err := p.store.UpdatePluginsInventoryCache(p.entityKey)
	if err != nil {
		prlog.WithFields(logrus.Fields{
			"entityKey": p.entityKey,
		}).WithError(err).Error("failed to update inventory cache")
		return
	}
}

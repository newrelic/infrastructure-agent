// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"github.com/sirupsen/logrus"
	"sync"
)

type InMemoryEntriesHook struct {
	lock    sync.RWMutex
	entries []logrus.Entry
	levels  []logrus.Level
}

func NewInMemoryEntriesHook(levels []logrus.Level) *InMemoryEntriesHook {
	return &InMemoryEntriesHook{levels: levels}
}

func (h *InMemoryEntriesHook) Levels() []logrus.Level {
	return h.levels
}

func (h *InMemoryEntriesHook) GetEntries() []logrus.Entry {
	h.lock.RLock()
	defer h.lock.RUnlock()
	entries := make([]logrus.Entry, len(h.entries))
	copy(entries, h.entries)
	return entries
}

func (h *InMemoryEntriesHook) Fire(entry *logrus.Entry) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.entries = append(h.entries, *entry)
	return nil
}

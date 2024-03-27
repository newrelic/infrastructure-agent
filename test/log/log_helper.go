// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"errors"
	"regexp"
	"sync"

	"github.com/sirupsen/logrus" //nolint:depguard
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

func (h *InMemoryEntriesHook) EntryWithMessageExists(entry *regexp.Regexp) bool {
	for _, e := range h.GetEntries() {
		if entry.MatchString(e.Message) {
			return true
		}
	}

	return false
}

func (h *InMemoryEntriesHook) EntryWithErrorExists(err error) bool {
	for _, e := range h.GetEntries() {
		//nolint:forcetypeassert
		if errors.Is(err, e.Data["error"].(error)) {
			return true
		}
	}

	return false
}

func (h *InMemoryEntriesHook) Fire(entry *logrus.Entry) error {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.entries = append(h.entries, *entry)

	return nil
}

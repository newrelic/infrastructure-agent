// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package feature_flags

import (
	"sync"

	"github.com/pkg/errors"
)

var (
	ErrFeatureFlagAlreadyExists = errors.New("feature flag already exists")
)

type Setter interface {
	SetFeatureFlag(name string, enabled bool) error
}

type Retriever interface {
	GetFeatureFlag(name string) (enabled, exists bool)
}

// Manager allows you to add or get feature flags.
type Manager interface {
	Setter
	Retriever
}

type FeatureFlags struct {
	features map[string]bool
	lock     sync.Mutex
}

func NewManager(initialFeatureFlags map[string]bool) *FeatureFlags {
	ff := map[string]bool{}

	if initialFeatureFlags != nil {
		for key, value := range initialFeatureFlags {
			ff[key] = value
		}
	}

	return &FeatureFlags{
		features: ff,
		lock:     sync.Mutex{},
	}
}

// SetFeatureFlag adds a new FF to the config. FFs can be defined in the agent's config file or they may come from the
// Command Channel. As the command channel runs asynchronously, we need to lock the feature flags.
func (f *FeatureFlags) SetFeatureFlag(name string, enabled bool) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	// if the FF exists, it means either it's in the config file or it has been set already by the CommandChannel.
	// In either case, do not modify the previous value
	if _, ok := f.features[name]; ok {
		return ErrFeatureFlagAlreadyExists
	}

	f.features[name] = enabled
	return nil
}

// GetFeatureFlag returns if a FF is enabled and exists
func (f *FeatureFlags) GetFeatureFlag(name string) (enabled, exists bool) {
	f.lock.Lock()
	defer f.lock.Unlock()

	enabled, exists = f.features[name]
	return
}

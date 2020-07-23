// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package feature_flags

import (
	"errors"
	"sync"
)

var (
	ErrFeatureFlagAlreadyExists = errors.New("feature flag already exists")
)

type Setter interface {
	// SetFeatureFlag enables or disables FF on the config if not already set.
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
	featuresFromCfg map[string]bool
	features        map[string]bool
	lock            sync.Mutex
}

func NewManager(initialFeatureFlags map[string]bool) *FeatureFlags {
	fInitial := map[string]bool{}
	fFromCfg := map[string]bool{}

	if initialFeatureFlags != nil {
		for key, value := range initialFeatureFlags {
			fInitial[key] = value
			fFromCfg[key] = value
		}
	}

	return &FeatureFlags{
		featuresFromCfg: fFromCfg,
		features:        fInitial,
		lock:            sync.Mutex{},
	}
}

// SetFeatureFlag adds a new FF to the config. FFs can be defined in the agent's config file or they may come from the
// Command Channel. As the command channel runs asynchronously, we need to lock the feature flags.
func (f *FeatureFlags) SetFeatureFlag(name string, enabled bool) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	// if the FF was provided by user config, that one prevails
	if _, ok := f.featuresFromCfg[name]; ok {
		return ErrFeatureFlagAlreadyExists
	}

	// value from command-channel equals current state
	if v, ok := f.features[name]; ok && v == enabled {
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

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package feature_flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_FeatureFlags_WithNoInitialFeatures(t *testing.T) {
	// GIVEN an empty feature flags
	f := NewManager(nil)

	// WHEN a feature flag is enabled
	err := f.SetFeatureFlag("test_enabled", true)
	assert.NoError(t, err)

	// WHEN a feature flag is disabled
	err = f.SetFeatureFlag("test_disabled", false)
	assert.NoError(t, err)

	// THEN check that the enabled ff exists and is enabled
	enabled, exists := f.GetFeatureFlag("test_enabled")
	assert.True(t, exists)
	assert.True(t, enabled)

	// THEN check that disabled ff exists and is disabled
	enabled, exists = f.GetFeatureFlag("test_disabled")
	assert.True(t, exists)
	assert.False(t, enabled)

	// THEN check that a made up ff doesn't exist
	_, exists = f.GetFeatureFlag("made_up_ff")
	assert.False(t, exists)
}

func TestConfig_FeatureFlags_WithInitialFeatures(t *testing.T) {
	// GIVEN some initial feature flags
	initialFeatureFlags := map[string]bool{
		"foo": true,
	}

	// GIVEN a feature flags instance initialized with feature flags
	f := NewManager(initialFeatureFlags)

	// WHEN disabling an existing feature flag
	err := f.SetFeatureFlag("foo", false)

	// THEN check that the feature hasn't been added or modified because it already exists
	assert.EqualError(t, err, ErrFeatureFlagAlreadyExists.Error())

	// THEN check that the feature flag keeps the initial value
	enabled, _ := f.GetFeatureFlag("foo")
	assert.True(t, enabled)
}

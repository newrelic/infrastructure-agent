// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConditionIsEvaluatedOnlyWhenFeatureIsEnabled(t *testing.T) {
	EnableOn([]string{"feature1", "feature2"})

	conditionACalled := false
	conditionBCalled := false

	conditionA := func() bool {
		conditionACalled = true
		return true
	}

	conditionB := func() bool {
		conditionBCalled = true
		return true
	}

	On(conditionA, Feature("feature666"), "")
	On(conditionB, Feature("feature1"), "")

	assert.False(t, conditionACalled)
	assert.True(t, conditionBCalled)
}

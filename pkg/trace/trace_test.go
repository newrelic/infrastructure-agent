// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package trace

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var buffer bytes.Buffer

func TestMain(m *testing.M) {
	// features to enable for tests
	EnableOn([]string{"feature1", "feature2"})

	// mocked logger to get the actual output
	mockedLogger := logrus.New()
	mockedLogger.Out = &buffer
	mockedLogger.SetLevel(logrus.TraceLevel)
	global.logger = mockedLogger

	os.Exit(m.Run())
}

func TestConditionIsEvaluatedOnlyWhenFeatureIsEnabled(t *testing.T) {
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

	On(conditionA, Feature("feature666"), nil, "")
	On(conditionB, Feature("feature1"), nil, "")

	assert.False(t, conditionACalled)
	assert.True(t, conditionBCalled)
}

func TestLogrusFields(t *testing.T) {
	expectedFields := map[string]interface{}{
		"component": "agentTests",
		"name":      "SystemSampler",
	}

	On(func() bool { return true }, Feature("feature1"), func() *logrus.Entry { return &logrus.Entry{Data: expectedFields} }, "")

	assert.Contains(t, buffer.String(), "component=agentTests")
	assert.Contains(t, buffer.String(), "name=SystemSampler")
}

func TestLogrusWithoutFields(t *testing.T) {
	On(func() bool { return true }, Feature("feature1"), func() *logrus.Entry { return &logrus.Entry{} }, "")

	assert.Contains(t, buffer.String(), "feature=feature1")
}

func TestLogrusWithoutEntry(t *testing.T) {
	On(func() bool { return true }, Feature("feature1"), nil, "")

	assert.Contains(t, buffer.String(), "feature=feature1")
}

func TestLogrusFieldsDisabled(t *testing.T) {
	On(func() bool { return true }, Feature("DisabledFeature"), func() *logrus.Entry {
		t.Log("this expensive operation should not be executed")
		t.Fail()
		return &logrus.Entry{}
	}, "")
}

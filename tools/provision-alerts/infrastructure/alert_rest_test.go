// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"provision-alerts/config"
	"testing"
	"time"
)

func TestFromConditionConfig(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	duration := rand.Int()
	threshold := float64(rand.Int())
	operator := "above"
	if duration%2 == 0 {
		operator = "below"
	}

	condition := config.ConditionConfig{
		Name:         "some random name",
		Metric:       "some random metric",
		Sample:       "some random sample",
		Duration:     duration,
		Threshold:    threshold,
		Operator:     operator,
		TemplateName: "random template name",
		NRQL:         "some random nrql",
	}

	nrqlConditionPayload := FromConditionConfig(condition)
	assert.Len(t, nrqlConditionPayload.NrqlCondition.Terms, 1)
	assert.Equal(t, fmt.Sprintf("%v", duration), nrqlConditionPayload.NrqlCondition.Terms[0].Duration)
	assert.Equal(t, fmt.Sprintf("%.1f", threshold), nrqlConditionPayload.NrqlCondition.Terms[0].Threshold)
	assert.Equal(t, "critical", nrqlConditionPayload.NrqlCondition.Terms[0].Priority)
	assert.Equal(t, "all", nrqlConditionPayload.NrqlCondition.Terms[0].TimeFunction)
	assert.Equal(t, operator, nrqlConditionPayload.NrqlCondition.Terms[0].Operator)
	assert.Equal(t, condition.Name, nrqlConditionPayload.NrqlCondition.Name)
	assert.Equal(t, condition.NRQL, nrqlConditionPayload.NrqlCondition.Nrql.Query)
	assert.Equal(t, true, nrqlConditionPayload.NrqlCondition.Enabled)
	assert.Equal(t, "static", nrqlConditionPayload.NrqlCondition.Type)
	assert.Equal(t, "single_value", nrqlConditionPayload.NrqlCondition.ValueFunction)
	assert.Equal(t, 259200, nrqlConditionPayload.NrqlCondition.ViolationTimeLimitSeconds)
}

func TestFromPolicyConfig(t *testing.T) {
	name := "some name"
	incidentPreference := "some incident preference"

	policyConfig := config.PolicyConfig{
		Name:               name,
		IncidentPreference: incidentPreference,
	}
	expectedPolicyPayload := PolicyPayload{
		Policy: PolicyDetailsPayload{
			Name:               name,
			IncidentPreference: incidentPreference,
		}}

	actualPolicyPayload := FromPolicyConfig(policyConfig)

	assert.Equal(t, expectedPolicyPayload, actualPolicyPayload)
}

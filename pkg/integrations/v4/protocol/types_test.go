// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"fmt"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
)

func TestEntityData_New(t *testing.T) {
	want := EventData{
		"eventType":            "InfrastructureEvent",
		"category":             "test-category",
		"summary":              "test-summary",
		"entity_name":          "test-entity_name",
		"format":               "test-format",
		"local_identity":       "test-local_identity",
		"local_details":        "test-local_details",
		"integrationUser":      "test-user",
		"entityKey":            "test-entity-key",
		"entityID":             "1234567890",
		"label.test-label-key": "test-label-value",
		"attr.format":          "from-attribute",
		"foo-attribute":        "test-foo",
		"bar-attribute":        "test-bar",
		"lorem-attribute":      "test-lorem",
		"region":               "eu-west",
	}

	e := EventData{
		"category":           "test-category", // integrations are able to override default category
		"summary":            "test-summary",
		"entity_name":        "test-entity_name",
		"format":             "test-format",
		"local_identity":     "test-local_identity",
		"local_details":      "test-local_details",
		"not_accepted_event": "test-ignore-attribute",
	}

	u := "test-user"

	l := map[string]string{
		"test-label-key": "test-label-value",
	}

	en := entity.Entity{
		Key: "test-entity-key",
		ID:  entity.ID(1234567890),
	}

	attributes := map[string]any{
		"format":          "from-attribute",
		"foo-attribute":   "test-foo",
		"bar-attribute":   "test-bar",
		"lorem-attribute": "test-lorem",
	}

	annotations := map[string]string{
		"region": "eu-west",
		// repeated: should not be added
		"category": "test-category",
	}

	n, err := NewEventData(
		WithEvents(e),
		WithIntegrationUser(u),
		WithLabels(l),
		WithEntity(en),
		WithAttributes(attributes),
		WithAnnotations(annotations),
	)

	assert.NoError(t, err)
	assertEventDataValues(t, want, n)

	// eventData should remain immutable, NewEventData should copy map values
	// Assert that maps values points to diff memory address
	assert.NotSame(t, &want, &n)
}

func TestEntityData_New_IgnoreHostnameAttribute(t *testing.T) {
	want := EventData{
		"eventType": "InfrastructureEvent",
		"category":  "notifications",
		"summary":   "test",
	}

	a := map[string]interface{}{
		"summary":  "test",
		"hostname": "test-hostname",
	}

	n, err := NewEventData(WithAttributes(a))

	assert.NoError(t, err)
	assertEventDataValues(t, want, n)
}

func TestEventData_New_MissingRequiredKey(t *testing.T) {
	_, err := NewEventData(WithEvents(EventData{}))

	assert.Error(t, err)
}

func assertEventDataValues(t *testing.T, want EventData, n EventData) {
	for key, value := range want {
		v, ok := n[key]
		if !ok {
			assert.Fail(t, fmt.Sprintf("Missing key:%s in build it EventData.", key))
		}
		assert.Equal(t, value, v)
	}
}

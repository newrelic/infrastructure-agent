// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package bulk

import (
	"encoding/json"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/stretchr/testify/assert"
)

func fakePostDeltaBody() inventoryapi.PostDeltaBody {
	isAgent := false
	delta := inventoryapi.RawDelta{}
	err := json.Unmarshal([]byte(`{
		"source": "testFake",
		"id": 543210,
		"timestamp": 123456789,
		"diff": {"some": {"nice": "json"}, "is": "here"},
		"full_diff": true
	}`), &delta)
	if err != nil {
		panic(err) // that should never happen!
	}
	return inventoryapi.PostDeltaBody{
		ExternalKeys: []string{"test_external_key"},
		IsAgent:      &isAgent,
		Deltas:       []*inventoryapi.RawDelta{&delta},
	}
}

func TestBuffer_AddLimits(t *testing.T) {
	deltaBody := fakePostDeltaBody()
	bodySize, err := sizeOf(&deltaBody)
	assert.Nil(t, err)

	// Given an empty Buffer with a limited size
	buffer := NewBuffer(5 * bodySize / 2)

	// You can add elements as long as they fit in the buffer limits
	assert.Nil(t, buffer.Add("entity1", deltaBody))
	assert.Nil(t, buffer.Add("entity2", deltaBody))

	// But it will fail if you try to add an element that doesn't fit
	assert.NotNil(t, buffer.Add("entity3", deltaBody))

	// and won't add the element that made the addition process fail
	assert.Nil(t, buffer.Get("entity3"))

	// And when you clear the buffer
	buffer.Clear()

	// You can now continue adding elements
	assert.Nil(t, buffer.Add("entity3", deltaBody))

	// And the buffer has discarded the old elements, containing the new ones
	assert.Nil(t, buffer.Get("entity1"))
	assert.Nil(t, buffer.Get("entity2"))
	assert.NotNil(t, buffer.Get("entity3"))
}

func TestBuffer_AsSlice(t *testing.T) {
	deltaBody := fakePostDeltaBody()

	// Given an empty Buffer with a limited size
	buffer := NewBuffer(10000)

	// With a number of delta bodies inside
	assert.Nil(t, buffer.Add("entity1", deltaBody))
	assert.Nil(t, buffer.Add("entity2", deltaBody))
	assert.Nil(t, buffer.Add("entity3", deltaBody))
	assert.Equal(t, 3, buffer.Entries())

	// The deltas can be retrieved as a slice
	allDeltas := buffer.AsSlice()
	assert.Equal(t, 3, len(allDeltas))
}

func TestBuffer_AddDuplicateEntry(t *testing.T) {
	deltaBody := fakePostDeltaBody()

	// Given an empty Buffer
	buffer := NewBuffer(10000)

	// That already contains data for some entities
	assert.Nil(t, buffer.Add("entity1", deltaBody))
	assert.Nil(t, buffer.Add("entity2", deltaBody))

	// When trying to add new data for an entity that is already buffered
	// the function returns error
	assert.NotNil(t, buffer.Add("entity1", deltaBody))
	// and has not included any new entry
	assert.Equal(t, 2, buffer.Entries())

	// And when the buffer is cleared again
	buffer.Clear()
	// The entity data can be added again
	assert.Nil(t, buffer.Add("entity1", deltaBody))
	assert.Equal(t, 1, buffer.Entries())

}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// Package bulk provides utilities and mechanism for sending bulk inventories to the backend, including management of
// sources from different entities and dividing payloads when they do not fit into the size limits at backend side
//
package bulk

import (
	"encoding/json"
	"fmt"

	"github.com/newrelic/infrastructure-agent/pkg/backend/inventoryapi"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

// Buffer helps creating payloads from multiple PostDeltaBody instances and control their size
type Buffer struct {
	capacity    int
	currentSize int
	contents    map[entity.Key]inventoryapi.PostDeltaBody
}

// NewBuffer creates a new bulk buffer
func NewBuffer(capacity int) Buffer {
	return Buffer{
		capacity:    capacity,
		currentSize: 0,
		contents:    map[entity.Key]inventoryapi.PostDeltaBody{},
	}
}

// Add a given payload to buffers and associates it with the given entity. It returns error if:
// - There is already a payload for the entity
// - The body can't be marshaled into a JSON
// - The object is too big for the free space of the buffer
func (b *Buffer) Add(ent entity.Key, body inventoryapi.PostDeltaBody) error {
	if _, ok := b.contents[ent]; ok {
		return fmt.Errorf("entity already added: %q", ent)
	}
	bodySize, err := sizeOf(&body)
	if err != nil {
		return err
	}
	if b.currentSize+bodySize > b.capacity {
		return fmt.Errorf("delta for entity %q does not fit into the request limits. "+
			"Free space: %d. Delta size: %d", ent, b.capacity-b.currentSize, bodySize)
	}
	b.contents[ent] = body
	b.currentSize += bodySize
	return nil
}

// Get returns the PostDeltaBody buffered for the given entity ID, or nil if there is no PostDeltaBody for such entity
func (b Buffer) Get(ent entity.Key) *inventoryapi.PostDeltaBody {
	body, ok := b.contents[ent]
	if !ok {
		return nil
	}
	return &body
}

// Clear empties the buffer
func (b *Buffer) Clear() {
	b.currentSize = 0
	b.contents = map[entity.Key]inventoryapi.PostDeltaBody{}
}

// AsSlice returns the PostDeltaBody entries as a slice
func (b Buffer) AsSlice() []inventoryapi.PostDeltaBody {
	postDeltaMap := b.contents
	contents := make([]inventoryapi.PostDeltaBody, 0, len(postDeltaMap))
	for _, postDelta := range postDeltaMap {
		contents = append(contents, postDelta)
	}
	return contents
}

// Entries returns the number of entries that are buffered
func (b Buffer) Entries() int {
	return len(b.contents)
}

// sizeOf calculates the size of a PostDeltaBody by unmarshaling it into a byte array.
// It is not an efficient way of doing it because it generates unnecessary amounts of memory. It should
// be replaced by any other JSON size estimation mechanism which, if not 100% accurate, would provide
// a size equal or larger to the actual size, so we will be sure that the estimated payload fits in a
// size-limited request
func sizeOf(body *inventoryapi.PostDeltaBody) (int, error) {
	buffer, err := json.Marshal(body)
	if err != nil {
		return -1, err
	}
	return len(buffer), nil
}

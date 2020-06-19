// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package lru

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// LRU cache testing is based on Google Groupcache's LRU implementation, distributed under Apache License 2.0 in the
// following repository: https://github.com/golang/groupcache

type simpleStruct struct {
	int
	string
}

type complexStruct struct {
	int
	simpleStruct
}

var getTests = []struct {
	name       string
	keyToAdd   interface{}
	keyToGet   interface{}
	expectedOk bool
}{
	{"string_hit", "myKey", "myKey", true},
	{"string_miss", "myKey", "nonsense", false},
	{"simple_struct_hit", simpleStruct{1, "two"}, simpleStruct{1, "two"}, true},
	{"simeple_struct_miss", simpleStruct{1, "two"}, simpleStruct{0, "noway"}, false},
	{"complex_struct_hit", complexStruct{1, simpleStruct{2, "three"}},
		complexStruct{1, simpleStruct{2, "three"}}, true},
}

func TestGet(t *testing.T) {
	for _, tt := range getTests {
		lru := New()
		lru.Add(tt.keyToAdd, 1234)
		val, ok := lru.Get(tt.keyToGet)
		if ok != tt.expectedOk {
			t.Fatalf("%s: cache hit = %v; want %v", tt.name, ok, !ok)
		} else if ok && val != 1234 {
			t.Fatalf("%s expected get to return 1234 but got %v", tt.name, val)
		}
	}
}

func TestRemove(t *testing.T) {
	lru := New()
	lru.Add("myKey", 1234)
	if val, ok := lru.Get("myKey"); !ok {
		t.Fatal("TestRemove returned no match")
	} else if val != 1234 {
		t.Fatalf("TestRemove failed.  Expected %d, got %v", 1234, val)
	}

	lru.Remove("myKey")
	if _, ok := lru.Get("myKey"); ok {
		t.Fatal("TestRemove returned a removed entry")
	}
}

func TestRemoveUntilLen(t *testing.T) {
	lru := New()
	lru.Add("Key1", true)
	lru.Add("Key2", false)
	lru.Add("Key3", true)
	lru.Add("Key4", false)

	assert.Equal(t, lru.Len(), 4)

	val, ok := lru.Get("Key1")
	assert.True(t, val.(bool))
	assert.True(t, ok)

	val, ok = lru.Get("Key2")
	assert.False(t, val.(bool))
	assert.True(t, ok)

	val, ok = lru.Get("Key3")
	assert.True(t, val.(bool))
	assert.True(t, ok)

	val, ok = lru.Get("Key4")
	assert.False(t, val.(bool))
	assert.True(t, ok)

	lru.RemoveUntilLen(2)

	assert.Equal(t, lru.Len(), 2)

	_, ok = lru.Get("Key1")
	assert.False(t, ok)

	_, ok = lru.Get("Key2")
	assert.False(t, ok)

	val, ok = lru.Get("Key3")
	assert.True(t, val.(bool))
	assert.True(t, ok)

	val, ok = lru.Get("Key4")
	assert.False(t, val.(bool))
	assert.True(t, ok)

	lru.Add("NewestKey", true)
	lru.RemoveUntilLen(1)

	_, ok = lru.Get("Key3")
	assert.False(t, ok)

	_, ok = lru.Get("Key4")
	assert.False(t, ok)

	val, ok = lru.Get("NewestKey")
	assert.True(t, val.(bool))
	assert.True(t, ok)
}

func TestRemoveUntilLen_Add(t *testing.T) {
	lru := New()
	lru.Add("Key1", true)
	lru.Add("Key2", false)
	lru.Add("Key3", true)
	lru.Add("Key4", false)

	assert.Equal(t, lru.Len(), 4)

	lru.Add("Key3", false)
	lru.Add("Key2", true)

	lru.RemoveUntilLen(2)

	assert.Equal(t, lru.Len(), 2)

	_, ok := lru.Get("Key1")
	assert.False(t, ok)

	_, ok = lru.Get("Key4")
	assert.False(t, ok)

	val, ok := lru.Get("Key3")
	assert.True(t, ok)
	assert.False(t, val.(bool))

	val, ok = lru.Get("Key2")
	assert.True(t, ok)
	assert.True(t, val.(bool))

	lru.Add("NewestKey", true)
	lru.RemoveUntilLen(1)

	_, ok = lru.Get("Key2")
	assert.False(t, ok)

	_, ok = lru.Get("Key2")
	assert.False(t, ok)

	val, ok = lru.Get("NewestKey")
	assert.True(t, val.(bool))
	assert.True(t, ok)
}

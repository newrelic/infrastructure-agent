// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFields_Key(t *testing.T) {
	f := Fields{
		Name: "foo",
		Type: "bar",
	}

	k, err := f.Key()

	assert.NoError(t, err)
	assert.Equal(t, k.String(), "bar:foo")
}

func TestKey_IsEmpty(t *testing.T) {
	assert.Equal(t, Key(""), EmptyKey)

	assert.True(t, EmptyKey.IsEmpty())
}

func TestKey_WithAttributes(t *testing.T) {
	f := Fields{
		Name: "foo",
		Type: "bar",
		IDAttributes: IDAttributes{
			{
				Key:   "env",
				Value: "prod",
			},
			{
				Key:   "srv",
				Value: "auth",
			},
		},
	}

	k, err := f.Key()

	assert.NoError(t, err)
	assert.Equal(t, k.String(), "bar:foo:env=prod:srv=auth")
}

func TestKey_DuplicatedAttributesAreDroppedUsingLastVale(t *testing.T) {
	f := Fields{
		Name: "foo",
		Type: "bar",
		IDAttributes: IDAttributes{
			{
				Key:   "env",
				Value: "foo",
			},
			{
				Key:   "env",
				Value: "bar",
			},
		},
	}

	k, err := f.Key()

	assert.NoError(t, err)
	assert.Equal(t, "bar:foo:env=bar", k.String())
}

func TestKey_IDAttributesWithEmptyKeyAreDropped(t *testing.T) {
	f := Fields{
		Name: "foo",
		Type: "bar",
		IDAttributes: IDAttributes{
			{
				Key:   "",
				Value: "baz",
			},
			{
				Key:   "foo",
				Value: "bar",
			},
		},
	}

	k, err := f.Key()

	assert.NoError(t, err)
	assert.Equal(t, "bar:foo:foo=bar", k.String())
}

func TestKey_AttributesAreConvertedToLowerCase(t *testing.T) {
	f := Fields{
		Name: "Foo",
		Type: "Bar",
		IDAttributes: IDAttributes{
			{
				Key:   "Env",
				Value: "Prod",
			},
		},
	}

	k, err := f.Key()

	assert.NoError(t, err)
	assert.Equal(t, "Bar:Foo:env=prod", k.String())
}

func TestEntity_AttributesAreSortedByKey(t *testing.T) {

	attr1 := IDAttribute{
		Key:   "aaa",
		Value: "x",
	}
	attr2 := IDAttribute{
		Key:   "bbb",
		Value: "x",
	}
	attr3 := IDAttribute{
		Key:   "ccc",
		Value: "x",
	}
	attr4 := IDAttribute{
		Key:   "ddd",
		Value: "x",
	}
	attr5 := IDAttribute{
		Key:   "zzz",
		Value: "x",
	}

	expected := "type:name:aaa=x:bbb=x:ccc=x:ddd=x:zzz=x"
	attributes := [][]IDAttribute{
		{attr1, attr2, attr3, attr4, attr5},
		{attr2, attr3, attr4, attr1, attr5},
		{attr1, attr5, attr4, attr3, attr2},
		{attr5, attr4, attr3, attr2, attr1},
	}
	for _, attrs := range attributes {
		f := Fields{
			Name:         "name",
			Type:         "type",
			IDAttributes: attrs,
		}
		k, err := f.Key()
		assert.NoError(t, err)
		assert.Equal(t, expected, k.String())
	}
}

func TestKey_EmptyName(t *testing.T) {
	f := Fields{
		Type: "bar",
		IDAttributes: IDAttributes{
			{
				Key:   "env",
				Value: "prod",
			},
			{
				Key:   "srv",
				Value: "auth",
			},
		},
	}
	k, err := f.Key()
	assert.NoError(t, err)
	assert.Equal(t, EmptyKey, k)
}

func TestKey_EmptyType(t *testing.T) {
	f := Fields{
		Name: "foo",
	}
	_, err := f.Key()
	assert.Error(t, err)
}

func TestKey_Empty(t *testing.T) {
	f := Fields{}
	k, err := f.Key()
	assert.NoError(t, err)
	assert.Equal(t, EmptyKey, k)
}

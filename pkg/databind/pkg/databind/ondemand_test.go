// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplace_OnDemand_ByteSlice(t *testing.T) {
	// GIVEN a byte array with variable marks in the inner values
	template := []byte("Hello ${name.yours},\nMy name is ${name.mine}.\nGoodbye!")
	// WHEN they are replaced by a set of two discovered items and an OnDemand provider
	ctx := &Values{
		discov: []discovery.Discovery{
			{Variables: data.Map{"name.yours": "Fred"}},
			{Variables: data.Map{"name.yours": "Marc"}},
		}}
	ret, err := ReplaceBytes(ctx, template, Provided(func(name string) (value []byte, found bool) {
		if name == "name.mine" {
			return []byte("Anna"), true
		}
		return nil, false
	}))
	require.NoError(t, err)

	// THEN two replaced instances are returned
	require.Len(t, ret, 2)

	// AND both replaced instances have all the variables replaced according to the discovered items
	assert.Equal(t, []byte("Hello Fred,\nMy name is Anna.\nGoodbye!"), ret[0])
	assert.Equal(t, []byte("Hello Marc,\nMy name is Anna.\nGoodbye!"), ret[1])
}

func TestReplace_OnDemand(t *testing.T) {
	// GIVEN a complex structure with variable marks in the inner values
	type testStruct struct {
		URL string
	}
	myConfig := testStruct{
		URL: "http://${discovery.ip}:${discovery.port}/${dynamicPath}",
	}

	// WHEN it is replaced by a set of two discovered items and a dynamic ondemand provider
	ctx := &Values{discov: []discovery.Discovery{
		{Variables: data.Map{"discovery.ip": "1.2.3.4", "discovery.port": "8888", "hostname": "jarl"}},
		{Variables: data.Map{"discovery.ip": "5.6.7.8", "discovery.port": "1111", "hostname": "nopuedor"}},
	}}
	paths := []string{"path1", "path2"}
	pathNum := 0
	ret, err := Replace(ctx, myConfig, Provided(func(key string) (value []byte, found bool) {
		if key == "dynamicPath" {
			p := paths[pathNum]
			pathNum++
			return []byte(p), true
		}
		return nil, false
	}))
	require.NoError(t, err)

	// THEN two replaced instances are returned
	require.Len(t, ret, 2)

	// AND both replaced instances have all the variables replaced according to the discovered items
	ret0, ok := ret[0].Variables.(testStruct)
	require.Truef(t, ok, "the returned value must be of type %T. Was: %T", testStruct{}, ret0)
	assert.Equal(t, "http://1.2.3.4:8888/path1", ret0.URL)

	ret1, ok := ret[1].Variables.(testStruct)
	require.Truef(t, ok, "the returned value must be of type %T. Was: %T", testStruct{}, ret1)
	assert.Equal(t, "http://5.6.7.8:1111/path2", ret1.URL)
}

func TestFetchReplace_OnDemand_VarNotFound(t *testing.T) {
	// GIVEN a set of discovery values
	vals := Values{
		discov: []discovery.Discovery{{Variables: data.Map{"hello": "world", "bye": "you"}},
			{Variables: data.Map{"hello": "nen", "bye": "nano"}}},
	}

	// WHEN they are discovered against a given template with dynamically provided variables
	// and some of the variables can't be dynamically found
	template := map[string]string{
		"hello":    "${hello}",
		"bye":      "${bye}",
		"myVar":    "${myVar}",
		"mySecret": "${varNotFound}",
	}
	_, err := Replace(&vals, template, Provided(func(key string) (value []byte, found bool) {
		if key == "myVar" {
			return []byte("hello"), true
		}
		return nil, false
	}))

	// THEN an error is returned
	assert.Error(t, err)
}

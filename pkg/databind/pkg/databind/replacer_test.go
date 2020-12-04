// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/naming"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplace_NoVars_EmptyContext(t *testing.T) {
	// Given a configuration with no discoverable variables
	exampleConfig := struct {
		URL  string
		User string
		Num  int
	}{"http://www.google.com", "foo", 123}

	// When it is invoked with an empty context
	ret, err := Replace(&Values{}, exampleConfig)
	require.NoError(t, err)

	// The original configuration is returned, without modifications, as it is
	// assuming that the template is not subject to discovery
	require.Len(t, ret, 1)
	assert.Equal(t, exampleConfig, ret[0].Variables)
}

func TestReplace_Vars_EmptyContext(t *testing.T) {
	// Given a configuration with variables placeholder that do not match any discovery data
	type example struct {
		URL  string
		User string
		Num  int
	}
	cfg := example{"${myVar}", "${myOtherVar}", 123}

	// When it is invoked with an empty context
	ret, err := Replace(&Values{}, cfg)
	require.NoError(t, err)

	// No configuration is returned, as it is assuming no discovery matches
	require.Len(t, ret, 0)
}

func TestReplace_NoVars_PopulatedContext(t *testing.T) {
	// Given a configuration with no discoverable variables
	exampleConfig := struct {
		URL  string
		User string
		Num  int
	}{"http://www.google.com", "foo", 123}

	// When it is invoked with a populated context
	ret, err := Replace(&Values{discov: []discovery.Discovery{
		{Variables: data.Map{"hi": "ho"}},
		{Variables: data.Map{"hi": "ha"}}}}, exampleConfig)
	require.NoError(t, err)

	// The original configuration is returned, without modifications
	require.Len(t, ret, 1)
	assert.Equal(t, exampleConfig, ret[0].Variables)
}

func TestReplace_Map(t *testing.T) {
	// GIVEN a complex map with variable marks in the inner values
	type fakeStruct struct {
		Host string
	}
	myConfig := map[string]interface{}{
		"url": "http://${discovery.ip}:${discovery.port}/get",
		"labels": map[string]string{
			"hostname":  "${hostname}",
			"unchanged": "unchanged",
		},
		"nothingElse": struct{}{},
		"meta":        fakeStruct{"${discovery.ip} : ${hostname}"},
	}

	// WHEN they are replaced by a set of two discovered items
	ctx := &Values{discov: []discovery.Discovery{
		{Variables: data.Map{"discovery.ip": "1.2.3.4", "discovery.port": "8888", "hostname": "jarl"}},
		{Variables: data.Map{"discovery.ip": "5.6.7.8", "discovery.port": "1111", "hostname": "nopuedor"}},
	}}
	ret, err := Replace(ctx, myConfig)
	require.NoError(t, err)

	// THEN two replaced instances are returned
	require.Len(t, ret, 2)

	// AND both replaced instances have all the variables replaced according to the discovered items
	ret0, ok := ret[0].Variables.(map[string]interface{})
	require.Truef(t, ok, "the returned value must be of type map[string]interface{}. Was: %T", ret0)
	assert.Equal(t, "http://1.2.3.4:8888/get", ret0["url"])
	assert.Equal(t, struct{}{}, ret0["nothingElse"])
	lbls0, ok := ret0["labels"].(map[string]string)
	require.Truef(t, ok, "the inner labels must be of type map[string]string. Was: %T", lbls0)
	assert.Equal(t, "jarl", lbls0["hostname"])
	assert.Equal(t, "unchanged", lbls0["unchanged"])
	assert.Equal(t, ret0["meta"], fakeStruct{"1.2.3.4 : jarl"})

	ret1, ok := ret[1].Variables.(map[string]interface{})
	require.Truef(t, ok, "the returned value must be of type map[string]interface{}. Was: %T", ret1)
	assert.Equal(t, "http://5.6.7.8:1111/get", ret1["url"])
	assert.Equal(t, struct{}{}, ret1["nothingElse"])
	lbls1, ok := ret1["labels"].(map[string]string)
	require.Truef(t, ok, "the inner labels must be of type map[string]string. Was: %T", lbls1)
	assert.Equal(t, "nopuedor", lbls1["hostname"])
	assert.Equal(t, "unchanged", lbls1["unchanged"])
	assert.Equal(t, ret1["meta"], fakeStruct{"5.6.7.8 : nopuedor"})
}

func TestReplace_ByteSlice(t *testing.T) {
	// GIVEN a byte array with variable marks in the inner values
	template := []byte("Hello ${name.yours},\nMy name is ${name.mine}.\nGoodbye!")
	// WHEN they are replaced by a set of two discovered items
	ctx := &Values{
		vars: map[string]string{"name.mine": "Anna"},
		discov: []discovery.Discovery{
			{Variables: data.Map{"name.yours": "Fred"}},
			{Variables: data.Map{"name.yours": "Marc"}},
		}}
	ret, err := ReplaceBytes(ctx, template)
	require.NoError(t, err)

	// THEN two replaced instances are returned
	require.Len(t, ret, 2)

	// AND both replaced instances have all the variables replaced according to the discovered items
	assert.Equal(t, []byte("Hello Fred,\nMy name is Anna.\nGoodbye!"), ret[0])
	assert.Equal(t, []byte("Hello Marc,\nMy name is Anna.\nGoodbye!"), ret[1])
}

func TestReplace_Struct(t *testing.T) {
	// GIVEN a complex structure with variable marks in the inner values
	type testStruct struct {
		URL       string
		Labels    map[string]string
		Labels2   map[string]interface{}
		Forget    struct{ Value string }
		Unchanged string
		Slice     []string
	}
	myConfig := testStruct{
		URL: "http://${discovery.ip}:${discovery.port}/get",
		Labels: map[string]string{
			"hostname": "${hostname}",
		},
		Labels2: map[string]interface{}{
			"port": ":${discovery.port}",
		},
		Unchanged: "not-changed",
		Slice:     []string{"host: ${hostname}", "ip: ${discovery.ip}", "port: ${discovery.port}"},
	}
	myConfig.Forget.Value = "${discovery.ip}:${discovery.port}"

	// WHEN it is replaced by a set of two discovered items
	vals := &Values{discov: []discovery.Discovery{
		{Variables: data.Map{"discovery.ip": "1.2.3.4", "discovery.port": "8888", "hostname": "jarl"}},
		{Variables: data.Map{"discovery.ip": "5.6.7.8", "discovery.port": "1111", "hostname": "nopuedor"}},
	}}
	ret, err := Replace(vals, myConfig)
	require.NoError(t, err)

	// THEN two replaced instances are returned
	require.Len(t, ret, 2)

	// AND both replaced instances have all the variables replaced according to the discovered items
	ret0, ok := ret[0].Variables.(testStruct)
	require.Truef(t, ok, "the returned value must be of type %T. Was: %T", testStruct{}, ret0)
	assert.Equal(t, "http://1.2.3.4:8888/get", ret0.URL)
	assert.Equal(t, "jarl", ret0.Labels["hostname"])
	assert.Equal(t, ":8888", ret0.Labels2["port"])
	assert.Equal(t, "1.2.3.4:8888", ret0.Forget.Value)
	assert.Equal(t, "not-changed", ret0.Unchanged)
	assert.Equal(t, []string{"host: jarl", "ip: 1.2.3.4", "port: 8888"}, ret0.Slice)

	ret1, ok := ret[1].Variables.(testStruct)
	require.Truef(t, ok, "the returned value must be of type %T. Was: %T", testStruct{}, ret1)
	assert.Equal(t, "http://5.6.7.8:1111/get", ret1.URL)
	assert.Equal(t, "nopuedor", ret1.Labels["hostname"])
	assert.Equal(t, ":1111", ret1.Labels2["port"])
	assert.Equal(t, "5.6.7.8:1111", ret1.Forget.Value)
	assert.Equal(t, "not-changed", ret1.Unchanged)
	assert.Equal(t, []string{"host: nopuedor", "ip: 5.6.7.8", "port: 1111"}, ret1.Slice)
}

func TestFetchReplace_WithVars(t *testing.T) {
	// GIVEN a discovery source that returns 2 matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{
			{Variables: data.Map{"hello": "world", "bye": "you"}},
			{Variables: data.Map{"hello": "nen", "bye": "nano"}}}, nil
	}}
	// AND a set of variables defined by the user
	variable := func(value string) *gatherer {
		return &gatherer{fetch: func() (interface{}, error) {
			return value, nil
		}}
	}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables: map[string]*gatherer{
			"myVar":    variable("myValue"),
			"mySecret": variable("ssshh"),
		},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a given template
	template := map[string]string{
		"hello":    "${hello}",
		"bye":      "${bye}",
		"myVar":    "${myVar}",
		"mySecret": "${mySecret}",
	}
	repl, err := Replace(&vals, template)
	require.NoError(t, err)

	// THEN 2 matches are returned
	require.Len(t, repl, 2)

	// AND the replaced templates have different values for the different discovery matches
	require.IsType(t, map[string]string{}, repl[0].Variables)
	repl0 := repl[0].Variables.(map[string]string)
	assert.Equal(t, "world", repl0["hello"])
	assert.Equal(t, "you", repl0["bye"])

	require.IsType(t, map[string]string{}, repl[1].Variables)
	repl1 := repl[1].Variables.(map[string]string)
	assert.Equal(t, "nen", repl1["hello"])
	assert.Equal(t, "nano", repl1["bye"])

	// AND the replaced templates have the same value for the user-provided variables
	assert.Equal(t, "myValue", repl0["myVar"])
	assert.Equal(t, "ssshh", repl0["mySecret"])
	assert.Equal(t, "myValue", repl1["myVar"])
	assert.Equal(t, "ssshh", repl1["mySecret"])
}

func TestFetchReplace_ComplexVars(t *testing.T) {
	// GIVEN a variable that returns a complex object
	omelette := gatherer{fetch: func() (interface{}, error) {
		return map[string]interface{}{
			"receipt":  "omelette",
			"eggs":     3,
			"toppings": []interface{}{"garlic", "onion", "cheese"},
			"steps": map[string]string{
				"first":  "smash the eggs",
				"second": "put the toppings",
				"third":  "burn it!",
			},
		}, nil
	}}
	ctx := Sources{
		clock: time.Now,
		variables: map[string]*gatherer{
			"oml": &omelette,
		},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a given template
	//      with map- and array-like structured data
	template := map[string]string{
		"r":  "${oml.receipt}",
		"e":  "${oml.eggs}",
		"t0": "${oml.toppings[0]}",
		"t1": "${oml.toppings[1]}",
		"t2": "${oml.toppings[2]}",
		"s1": "${oml.steps.first}",
		"s2": "${oml.steps.second}",
		"s3": "${oml.steps.third}",
	}
	repl, err := Replace(&vals, template)
	require.NoError(t, err)

	// THEN 1 match is returned
	require.Len(t, repl, 1)

	// AND the templates are replaced by the structured data
	require.IsType(t, map[string]string{}, repl[0].Variables)
	repl0 := repl[0].Variables.(map[string]string)
	assert.Equal(t, "omelette", repl0["r"])
	assert.Equal(t, "3", repl0["e"])
	assert.Equal(t, "garlic", repl0["t0"])
	assert.Equal(t, "onion", repl0["t1"])
	assert.Equal(t, "cheese", repl0["t2"])
	assert.Equal(t, "smash the eggs", repl0["s1"])
	assert.Equal(t, "put the toppings", repl0["s2"])
	assert.Equal(t, "burn it!", repl0["s3"])
}

func TestFetchReplace_VarNotFound(t *testing.T) {
	// GIVEN a discovery source that returns 2 matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{
			{Variables: data.Map{"hello": "world", "bye": "you"}},
			{Variables: data.Map{"hello": "nen", "bye": "nano"}}}, nil
	}}
	// AND a set of variables defined by the user
	variable := func(value string) *gatherer {
		return &gatherer{fetch: func() (interface{}, error) {
			return value, nil
		}}
	}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables: map[string]*gatherer{
			"myVar": variable("myValue"),
		},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a given template, where some variables are not found
	template := map[string]string{
		"hello":    "${hello}",
		"bye":      "${bye}",
		"myVar":    "${myVar}",
		"mySecret": "${varNotFound}",
	}
	_, err = Replace(&vals, template)

	// THEN an error is returned
	assert.Error(t, err)
}

func TestFetchReplace_NoMatches_WithVars(t *testing.T) {
	// GIVEN a discovery source that returns no matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{}, nil
	}}
	// AND a set of variables defined by the user
	variable := func(value string) *gatherer {
		return &gatherer{fetch: func() (interface{}, error) {
			return value, nil
		}}
	}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables: map[string]*gatherer{
			"myVar": variable("myValue"),
		},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a given template
	template := map[string]string{
		"variable": "${myVar}",
	}
	matches, err := Replace(&vals, template)
	require.NoError(t, err)

	// THEN a match is returned, for the given variable
	assert.Len(t, matches, 1)
	assert.Equal(t, "myValue", matches[0].Variables.(map[string]string)["variable"])
}

func TestFetchReplace_MultipleMatches_NoVarsPlaceholders(t *testing.T) {
	// GIVEN a discovery source that returns multiple matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{
			{Variables: data.Map{"hello": "world", "bye": "you"}},
			{Variables: data.Map{"hello": "nen", "bye": "nano"}}}, nil
	}}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables:  map[string]*gatherer{},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a template without variable placeholders
	template := map[string]string{
		"variable": "hello",
	}
	matches, err := Replace(&vals, template)
	require.NoError(t, err)

	// THEN only one match is returned, with no values replaced
	assert.Len(t, matches, 1)
	assert.Equal(t, "hello", matches[0].Variables.(map[string]string)["variable"])
}

func TestFetchReplace_NoMatches_NoVarsPlaceholders(t *testing.T) {
	// GIVEN a discovery source that returns NO matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{}, nil
	}}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables:  map[string]*gatherer{},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a template that do not use variables
	template := map[string]string{
		"variable": "hello",
	}
	matches, err := Replace(&vals, template)
	require.NoError(t, err)

	// THEN only one match is returned, with the values replaced
	require.Len(t, matches, 1)
	assert.Equal(t, "hello", matches[0].Variables.(map[string]string)["variable"])
}

func TestFetchReplace_NoMatches_VarsPlaceholders(t *testing.T) {
	// GIVEN a discovery source that returns NO matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{}, nil
	}}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables:  map[string]*gatherer{},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a template that uses variables
	template := map[string]string{
		"variable": "${something}",
	}
	matches, err := Replace(&vals, template)

	// THEN NO errors are returned, but zero matches (as discovery just did not found
	// any target to apply
	require.NoError(t, err)
	assert.Len(t, matches, 0)
}

func TestFetchReplaceBytes_VarNotFound(t *testing.T) {
	// GIVEN a discovery source that returns 2 matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{{Variables: data.Map{"hello": "world", "bye": "you"}},
			{Variables: data.Map{"hello": "nen", "bye": "nano"}}}, nil
	}}
	// AND a set of variables defined by the user
	variable := func(value string) *gatherer {
		return &gatherer{fetch: func() (interface{}, error) {
			return value, nil
		}}
	}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables: map[string]*gatherer{
			"myVar": variable("myValue"),
		},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a given template, where some variables are not found
	_, err = ReplaceBytes(&vals, []byte("Hello ${hello} how ${myVar} ${varNotFound}?"))

	// THEN an error is returned
	assert.Error(t, err)
}

func TestFetchReplaceBytes_NoMatches_WithVars(t *testing.T) {
	// GIVEN a discovery source that returns no matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{}, nil
	}}
	// AND a set of variables defined by the user
	variable := func(value string) *gatherer {
		return &gatherer{fetch: func() (interface{}, error) {
			return value, nil
		}}
	}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables: map[string]*gatherer{
			"myVar": variable("myValue"),
		},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a given template
	matches, err := ReplaceBytes(&vals, []byte("${myVar}"))
	require.NoError(t, err)

	// THEN a match is returned, for the given variable
	assert.Len(t, matches, 1)
	assert.Equal(t, "myValue", string(matches[0]))
}

func TestFetchReplaceBytes_MultipleMatches_NoVarsPlaceholders(t *testing.T) {
	// GIVEN a discovery source that returns multiple matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{
			{Variables: data.Map{"hello": "world", "bye": "you"}},
			{Variables: data.Map{"hello": "nen", "bye": "nano"}}}, nil
	}}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables:  map[string]*gatherer{},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a template without variable placeholders
	matches, err := ReplaceBytes(&vals, []byte("hello"))
	require.NoError(t, err)

	// THEN only one match is returned, with no values replaced
	assert.Len(t, matches, 1)
	assert.Equal(t, "hello", string(matches[0]))
}

func TestFetchReplaceBytes_NoMatches_NoVarsPlaceholders(t *testing.T) {
	// GIVEN a discovery source that returns NO matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{}, nil
	}}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables:  map[string]*gatherer{},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a template that do not use variables
	matches, err := ReplaceBytes(&vals, []byte("hello"))
	require.NoError(t, err)

	// THEN only one match is returned, with the values replaced
	require.Len(t, matches, 1)
	assert.Equal(t, "hello", string(matches[0]))
}

func TestFetchReplaceBytes_NoMatches_VarsPlaceholders(t *testing.T) {
	// GIVEN a discovery source that returns NO matches
	discoverer := discoverer{fetch: func() ([]discovery.Discovery, error) {
		return []discovery.Discovery{}, nil
	}}
	ctx := Sources{
		clock:      time.Now,
		discoverer: &discoverer,
		variables:  map[string]*gatherer{},
	}
	vals, err := Fetch(&ctx)
	require.NoError(t, err)

	// WHEN this data is replaced against a template that uses variables
	matches, err := ReplaceBytes(&vals, []byte("${something}"))

	// THEN NO errors are returned, but zero matches (as discovery just did not found
	// any target to apply
	require.NoError(t, err)
	assert.Len(t, matches, 0)
}

func TestReplace_EntityRewrite(t *testing.T) {
	t.Parallel()
	variables := data.Map{"discovery." + data.IP: "1.2.3.4", "discovery." + data.ContainerID: "1234abc", "discovery." + data.PrivateIP: "2.3.4.5"}

	tests := []struct {
		name          string
		entityRewrite data.EntityRewrite
		match         string
		replaceField  string
	}{
		{
			name: "simple replace",
			entityRewrite: data.EntityRewrite{
				Action:       data.EntityRewriteActionReplace,
				Match:        naming.ToVariable(data.IP),
				ReplaceField: naming.ToVariable(data.ContainerID),
			},
			match:        "1.2.3.4",
			replaceField: "1234abc",
		},
		{
			name: "no match",
			entityRewrite: data.EntityRewrite{
				Action:       data.EntityRewriteActionReplace,
				Match:        naming.ToVariable(data.PrivateIP),
				ReplaceField: "uniqueId",
			},
			match:        "2.3.4.5",
			replaceField: "uniqueId",
		},
		{
			name: "complex",
			entityRewrite: data.EntityRewrite{
				Action:       data.EntityRewriteActionReplace,
				Match:        naming.ToVariable(data.IP),
				ReplaceField: naming.ToVariable(data.ContainerID) + ":base:" + naming.ToVariable(data.PrivateIP),
			},
			match:        "1.2.3.4",
			replaceField: "1234abc:base:2.3.4.5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Values{
				discov: []discovery.Discovery{
					{
						Variables:      variables,
						EntityRewrites: []data.EntityRewrite{tt.entityRewrite},
					},
				},
			}

			// GIVEN EntityRewrite config to replace ip with containerID
			myConfig := map[string]interface{}{}

			// WHEN they are replaced by a set of two discovered items
			ret, err := Replace(ctx, myConfig)
			require.NoError(t, err)

			// THEN one replaced EntityRewrite instances are returned
			require.Len(t, ret[0].EntityRewrites, 1)

			// AND replaced instance has all the variables replaced according to the discovered items
			ret0 := ret[0].EntityRewrites[0]
			assert.Equal(t, "replace", ret0.Action)
			assert.Equal(t, tt.match, ret0.Match)
			assert.Equal(t, tt.replaceField, ret0.ReplaceField)
		})
	}

	t.Run("combined", func(t *testing.T) {
		entityRewrites := make([]data.EntityRewrite, len(tests))
		for idx, tt := range tests {
			entityRewrites[idx] = tt.entityRewrite
		}
		ctx := &Values{discov: []discovery.Discovery{
			{
				Variables:      data.Map{"discovery." + data.IP: "1.2.3.4", "discovery." + data.ContainerID: "1234abc", "discovery." + data.PrivateIP: "2.3.4.5"},
				EntityRewrites: entityRewrites,
			},
		}}

		// GIVEN EntityRewrite config to replace ip with containerID
		myConfig := map[string]interface{}{}

		// WHEN they are replaced by a set of two discovered items
		ret, err := Replace(ctx, myConfig)
		require.NoError(t, err)

		// THEN two replaced EntityRewrite instances are returned
		require.Len(t, ret[0].EntityRewrites, len(tests))

		for idx, tt := range tests {
			// AND both replaced instances have all the variables replaced according to the discovered items
			ret := ret[0].EntityRewrites[idx]
			assert.Equal(t, "replace", ret.Action)
			assert.Equal(t, tt.match, ret.Match)
			assert.Equal(t, tt.replaceField, ret.ReplaceField)
		}
	})
}

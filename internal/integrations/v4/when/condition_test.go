// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package when

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestFileExists(t *testing.T) {
	// GIVEN an existing file
	f, err := ioutil.TempFile("", "conditions")
	require.NoError(t, err)

	// THEN the FileExists condition returns true
	assert.True(t, FileExists(f.Name())())
}

func TestFileExists_NotExist(t *testing.T) {
	// FileExists returns false if the passed file does not exist
	assert.False(t, FileExists("some-unexisting-file")())
}

func TestFileExists_IsDir(t *testing.T) {
	// GIVEN an existing directory
	f, err := ioutil.TempDir("", "conditions")
	require.NoError(t, err)

	// THEN the FileExists condition returns false,
	// as it exist but is not a file but a directory
	assert.False(t, FileExists(f)())
}

func TestEnvExists(t *testing.T) {
	testCases := map[string]struct {
		name      string
		assertion func(assert.TestingT, bool, ...interface{}) bool
		condition map[string]string
	}{
		"all match returns true": {
			assertion: assert.True,
			condition: map[string]string{"FOO": "foo", "BAR": "bar"},
		},
		"one mismatch returns false": {
			assertion: assert.False,
			condition: map[string]string{"FOO": "foo", "BAR": "baz"},
		},
		"no condition returns true": {
			assertion: assert.True,
			condition: map[string]string{},
		},
		"no env var is found returns false": {
			assertion: assert.False,
			condition: map[string]string{"BAZ": "baz"},
		},
		"value case missmatch returns false": {
			assertion: assert.False,
			condition: map[string]string{"FOO": "FOO"},
		},
	}
	os.Setenv("FOO", "foo")
	defer os.Unsetenv("FOO")
	os.Setenv("BAR", "bar")
	defer os.Unsetenv("BAR")
	for k, testCase := range testCases {
		t.Run(k, func(t *testing.T) {
			testCase.assertion(t, EnvExists(testCase.condition)())
		})
	}
}

func TestAll(t *testing.T) {
	trueFunc := func() bool { return true }
	falseFunc := func() bool { return false }

	type testCase struct {
		description string
		conditions  []Condition
		expected    bool
	}
	cases := []testCase{{
		description: "empty conditions should return true", // no conditions, nothing should avoid executing the integration
		conditions:  []Condition{},
		expected:    true,
	}, {
		description: "if conditions are true, should return true",
		conditions:  []Condition{trueFunc, trueFunc},
		expected:    true,
	}, {
		description: "if only one condition is false, should return false",
		conditions:  []Condition{trueFunc, falseFunc, trueFunc, trueFunc},
		expected:    false,
	}}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			assert.Equal(t, tc.expected, All(tc.conditions...))
		})
	}
}

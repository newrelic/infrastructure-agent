// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package http

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainsLocalhost(t *testing.T) {
	tests := []string{
		"localhost:123",
		"x:127.0.0.1:123",
		"x:127.0.0.123:123",
		"a:LOCAlhost:1",
		"::1",
		// The following could cause false positives
		"a:::1:b",
		"a:::1:b",
		"::1:b",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			assert.True(t, ContainsLocalhost(test))
		})
	}
}

func TestDoesNotContainsLocalhost(t *testing.T) {
	tests := []string{
		"local:123",
		"x:128.0.0.1:123",
		"a:HOST:1",
		"a:2001:db8::1:b",
		"a:20:A::1:db8:1:bas",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			assert.False(t, ContainsLocalhost(test))
		})
	}
}

func TestIsLocalhost(t *testing.T) {
	tests := []string{"localhost", "127.0.0.1", "127.0.0.123", "LOCALHOST", "::1"}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			assert.True(t, IsLocalhost(test))
		})
	}
}

func TestReplaceLocalhost(t *testing.T) {
	tests := map[string]string{
		"localhost:123":     "foo:123",
		"x:127.0.0.1:123":   "x:foo:123",
		"x:127.0.0.123:123": "x:foo:123",
		"a:LOCALHOST:1":     "a:foo:1",
		"a:::1:1":           "a:foo:1",
		"::1:b":             "foo:b",
		"::1":               "foo",
	}

	for src, expected := range tests {
		t.Run(fmt.Sprintf("src: %s - expec: %s", src, expected), func(t *testing.T) {
			assert.Equal(t, expected, ReplaceLocalhost(src, "foo"))
		})
	}
}

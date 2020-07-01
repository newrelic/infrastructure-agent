// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatcher_All(t *testing.T) {
	cm, err := NewMatcher(map[string]string{
		"container":     "hello",
		"label.version": "/^2\\./",
	})
	require.NoError(t, err)

	assert.True(t, cm.All(map[string]string{
		"something":     "dontmatter",
		"container":     "hello",
		"label.version": "2.3.4",
	}))

	assert.False(t, cm.All(map[string]string{
		"container":     "hello",
		"label.version": "v2.3.4 shouldn't work because 2 is not the first character",
	}))
}

func TestMatcher_CantInstantiate(t *testing.T) {
	_, err := NewMatcher(map[string]string{
		"container":     "hello",
		"label.version": "/^2\\./",
		"label.value":   "/[invalid regex/",
	})
	assert.Error(t, err)
}

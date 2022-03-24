// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeaturesCache_Update(t *testing.T) {
	c := FeaturesCache{
		"foo": "path_foo",
		"bar": "path_bar",
	}

	cNew := FeaturesCache{
		"baz": "path_baz",
		"foo": "path_bar",
	}

	c.Update(cNew)

	assert.Equal(t, FeaturesCache{
		"baz": "path_baz",
		"bar": "path_bar",
		"foo": "path_bar",
	}, c)
}

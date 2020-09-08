// Copyright 2019 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	"testing"
)

func TestCompress(t *testing.T) {
	input := "this is the input string that needs to be compressed"
	buf, err := Compress([]byte(input))
	require.NoError(t, err)
	back, err := Uncompress(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, input, string(back))
}

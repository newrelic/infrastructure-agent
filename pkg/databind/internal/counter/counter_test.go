// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package counter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByKind_Count(t *testing.T) {
	bk := ByKind{}
	require.Equal(t, 0, bk.Count("foo"))
	require.Equal(t, 1, bk.Count("foo"))
	require.Equal(t, 0, bk.Count("bar"))
	require.Equal(t, 2, bk.Count("foo"))
	require.Equal(t, 0, bk.Count("tralara"))
}

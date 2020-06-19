// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package testhelpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventually_Error(t *testing.T) {
	t.Skip("We skip this test, as it is expected to fail")
	Eventually(t, 10*time.Millisecond, func(t require.TestingT) {
		require.True(t, false)
	})
}

func TestEventually_Fail(t *testing.T) {
	t.Skip("We skip this test, as it is expected to fail")
	Eventually(t, 10*time.Millisecond, func(t require.TestingT) {
		t.FailNow()
	})
}

func TestEventually_Timeout(t *testing.T) {
	t.Skip("We skip this test, as it is expected to fail")
	Eventually(t, 10*time.Millisecond, func(t require.TestingT) {
		time.Sleep(5 * time.Second)
	})
}

func TestEventually_Success(t *testing.T) {
	num := 3
	Eventually(t, 5*time.Second, func(t require.TestingT) {
		require.Equal(t, 0, num)
		num--
	})
}

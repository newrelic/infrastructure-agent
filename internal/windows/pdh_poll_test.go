// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//+build windows
//+build amd64

package nrwin

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPdhPoll(t *testing.T) {
	// Given a PDH poller
	const freeSpacePct = "\\LogicalDisk(_Total)\\% Free Space"
	const freeMegs = "\\LogicalDisk(_Total)\\Free Megabytes"
	pdh, err := NewPdhPoll(logrus.Warnf, freeSpacePct, freeMegs)
	require.NoError(t, err)

	// When it polls the values
	values, err := pdh.Poll()
	require.NoError(t, err)

	// Those have sense
	assert.True(t, values[freeMegs] > 0, "%v <= 0", values[freeMegs])
	assert.True(t, values[freeSpacePct] > 0, "%v <= 0", values[freeSpacePct])
	assert.True(t, values[freeSpacePct] <= 100, "%v > 100", values[freeSpacePct])

	// And when it is not needed anymore
	// it can be closed
	err = pdh.Close()
	assert.NoError(t, err)
}

func TestPdhPoll_WrongMetrics(t *testing.T) {
	// Given a set of metric names with some of them wrong
	const goodMetric = "\\LogicalDisk(_Total)\\Free Megabytes"
	const wrongMetric = "\\LogicalDisk(_Total)\\Doughnuts"

	// A PDH cannot be instantiated
	_, err := NewPdhPoll(logrus.Warnf, goodMetric, wrongMetric)
	require.Error(t, err)
}

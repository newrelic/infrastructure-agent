/*
 * Copyright 2021 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_previousCanaryVersion(t *testing.T) {
	testCases := []struct {
		name             string
		referenceVersion string
		expectedVersion  string
	}{
		{
			name:             "patch version",
			referenceVersion: "1.33.2",
			expectedVersion:  "1.33.1",
		},
		{
			name:             "minor version",
			referenceVersion: "1.34.0",
			expectedVersion:  "1.33.2",
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			previousVersion, err := getPreviousVersion(testCase.referenceVersion)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedVersion, previousVersion)
		})
	}
}

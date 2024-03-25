// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestMetadataHarvesterDefault(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		envVars  map[string]string
		expected Metadata
	}{
		{
			name:     "no env vars empty metadata",
			expected: Metadata{},
			envVars:  map[string]string{},
		},
		{
			name:     "no host id expects empty metadata",
			envVars:  map[string]string{"one_env_var": "some value"},
			expected: Metadata{},
		},
		{
			name:     "NR_HOST_ID expects the host_id",
			envVars:  map[string]string{"one_env_var": "some value", "NR_HOST_ID": "the host id"},
			expected: Metadata{"host.id": "the host id"},
		},
	}

	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) { //nolint:paralelltest
			// Set environment variables
			for envVarKey, envVar := range testCase.envVars {
				err := os.Setenv(envVarKey, envVar)
				require.NoError(t, err)
			}

			harvester := MetadataHarvesterDefault{}
			actualMetadata, err := harvester.Harvest()
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, actualMetadata)

			// Unset environment variables
			for envVarKey := range testCase.envVars {
				err := os.Unsetenv(envVarKey)
				require.NoError(t, err)
			}
		})
	}
}

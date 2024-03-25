// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package identityapi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataHarvesterDefault(t *testing.T) {
	testCases := []struct {
		name     string
		envVars  map[string]string
		expected Metadata
	}{
		{
			name:     "no env vars empty metadata",
			expected: Metadata{},
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
		t.Run(testCase.name, func(t *testing.T) {
			// Set environment variables
			for envVarKey, envVar := range testCase.envVars {
				err := os.Setenv(envVarKey, envVar)
				assert.NoError(t, err)
			}

			harvester := MetadataHarvesterDefault{}
			actualMetadata, err := harvester.Harvest()
			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, actualMetadata)

			// Unset environment variables
			for envVarKey := range testCase.envVars {
				err := os.Unsetenv(envVarKey)
				assert.NoError(t, err)
			}
		})
	}
}

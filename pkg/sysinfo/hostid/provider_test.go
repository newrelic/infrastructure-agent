// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package hostid

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest
func TestMetadataHarvesterDefault(t *testing.T) {
	testCases := []struct {
		name           string
		envVars        map[string]string
		expectedHostID string
	}{
		{
			name:           "no env vars empty string",
			expectedHostID: "",
			envVars:        map[string]string{},
		},
		{
			name:           "no host id expects empty metadata",
			envVars:        map[string]string{"one_env_var": "some value"},
			expectedHostID: "",
		},
		{
			name:           "NR_HOST_ID expects the host_id",
			envVars:        map[string]string{"one_env_var": "some value", "NR_HOST_ID": "the host id"},
			expectedHostID: "the host id",
		},
	}

	//nolint:paralleltest
	for i := range testCases {
		testCase := testCases[i]
		t.Run(testCase.name, func(t *testing.T) {
			// Set environment variables
			for envVarKey, envVar := range testCase.envVars {
				err := os.Setenv(envVarKey, envVar)
				require.NoError(t, err)
			}

			provider := ProviderEnv{}
			actualHostID, err := provider.Provide()
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedHostID, actualHostID)

			// Unset environment variables
			for envVarKey := range testCase.envVars {
				err := os.Unsetenv(envVarKey)
				require.NoError(t, err)
			}
		})
	}
}

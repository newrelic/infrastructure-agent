// Copyright New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cmdapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAllowedToRunStopFromCmdAPI(t *testing.T) {
	tests := []struct {
		name            string
		integrationName string
		want            bool
	}{
		{"empty", "", true},
		{"allowed 1", "nri-process-discovery", false},
		{"allowed 2", "nri-lsi-java", false},
		{"not allowed", "nri-foo", true},
		{"extra suffix", "nri-lsi-java-foo", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsForbiddenToRunStopFromCmdAPI(tt.integrationName))
		})
	}
}

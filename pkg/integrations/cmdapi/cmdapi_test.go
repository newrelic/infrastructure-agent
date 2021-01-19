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
		{"empty", "", false},
		{"allowed", "nri-lsi-java", true},
		{"not allowed", "nri-foo", false},
		{"extra suffix", "nri-lsi-java-foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsAllowedToRunStopFromCmdAPI(tt.integrationName))
		})
	}
}

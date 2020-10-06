package protocol

import (
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest/protocol/protocoltest"
	"github.com/stretchr/testify/assert"
)

func TestIsCommandRequest(t *testing.T) {
	tests := []struct {
		name                  string
		line                  string
		wantIsCmdRequest      bool
		wantCmdRequestVersion Version
	}{
		{
			"empty case",
			"",
			false,
			VUnsupported,
		},
		{
			"garbage",
			"\n\r",
			false,
			VUnsupported,
		},
		{
			"simple case",
			protocoltest.Inline(protocoltest.FixtureFoo),
			true,
			V1,
		},
		{
			"version value should be string, not number",
			protocoltest.Inline(protocoltest.FixtureWrongIntegerVersion),
			false,
			VUnsupported,
		},
		{
			"wrong command content doesn't affect determining it's a cmd request and version",
			// still well formatted JSON is required
			protocoltest.Inline(protocoltest.FixtureWrongCommandShape),
			true,
			V1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIsCmdRequest, gotCmdRequestVersion := IsCommandRequest([]byte(tt.line))
			assert.Equal(t, tt.wantIsCmdRequest, gotIsCmdRequest)
			assert.Equal(t, tt.wantCmdRequestVersion, gotCmdRequestVersion)
		})
	}
}

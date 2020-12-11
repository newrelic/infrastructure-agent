package dm

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"gotest.tools/assert"
	"testing"
)

type fakeEmitter struct {
	wasCalled bool
}

func (e *fakeEmitter) Send(req fwrequest.FwRequest) {
	e.wasCalled = true
}

func TestEmitterWithFF_Send(t *testing.T) {

	tests := []struct {
		name                  string
		initialFeatureFlags   map[string]bool
		emitterWithRegister   bool
		emitterWithNoRegister bool
	}{
		{
			name:                  "register enabled",
			initialFeatureFlags:   map[string]bool{fflag.FlagDMRegisterEnable: true},
			emitterWithRegister:   true,
			emitterWithNoRegister: false,
		},
		{
			name:                  "register disabled",
			initialFeatureFlags:   map[string]bool{fflag.FlagDMRegisterEnable: false},
			emitterWithRegister:   false,
			emitterWithNoRegister: true,
		},
		{
			name:                  "register not set",
			initialFeatureFlags:   map[string]bool{},
			emitterWithRegister:   false,
			emitterWithNoRegister: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emitterWithRegister := &fakeEmitter{}
			emitterWithNoRegister := &fakeEmitter{}

			f := feature_flags.NewManager(tc.initialFeatureFlags)
			emitterWrapper := NewEmitterWithFF(emitterWithRegister, emitterWithNoRegister, f)

			req := fwrequest.NewFwRequest(integration.Definition{}, nil, nil, protocol.DataV4{})

			emitterWrapper.Send(req)

			assert.Equal(t, tc.emitterWithRegister, emitterWithRegister.wasCalled)
			assert.Equal(t, tc.emitterWithNoRegister, emitterWithNoRegister.wasCalled)
		})
	}
}

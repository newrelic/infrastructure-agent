package dm

import (
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
)

type emitterWithFF struct {
	ffManager           *feature_flags.FeatureFlags
	emitterWithRegister Emitter
	nonRegisterEmitter  Emitter
}

func (e *emitterWithFF) Send(req fwrequest.FwRequest) {
	if enabled, exists := e.ffManager.GetFeatureFlag(fflag.FlagDMRegisterEnable); exists && enabled {
		e.emitterWithRegister.Send(req)
	} else {
		e.nonRegisterEmitter.Send(req)
	}
}

func NewEmitterWithFF(
	emitterWithRegister Emitter,
	nonRegisterEmitter Emitter,
	ffManager *feature_flags.FeatureFlags) Emitter {

	return &emitterWithFF{
		ffManager:           ffManager,
		emitterWithRegister: emitterWithRegister,
		nonRegisterEmitter:  nonRegisterEmitter,
	}
}

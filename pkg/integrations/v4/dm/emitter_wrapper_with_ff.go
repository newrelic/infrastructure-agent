package dm

import (
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/backend/identityapi"
	"github.com/newrelic/infrastructure-agent/pkg/fwrequest"
)

type emitterWithFF struct {
	ffManager          *feature_flags.FeatureFlags
	emitter            Emitter
	nonRegisterEmitter Emitter
}

func (e *emitterWithFF) Send(req fwrequest.FwRequest) {
	if enabled, exists := e.ffManager.GetFeatureFlag(fflag.FlagDMRegisterEnable); exists && enabled {
		e.emitter.Send(req)
	} else {
		e.nonRegisterEmitter.Send(req)
	}
}

func NewEmitterWithFF(
	agentContext agent.AgentContext,
	dmSender MetricsSender,
	registerClient identityapi.RegisterClient,
	ffManager *feature_flags.FeatureFlags) Emitter {

	return &emitterWithFF{
		ffManager:          ffManager,
		emitter:            NewEmitter(agentContext, dmSender, registerClient),
		nonRegisterEmitter: NewNonRegisterEmitter(agentContext, dmSender),
	}
}

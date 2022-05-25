// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"context"
	"github.com/newrelic/infrastructure-agent/pkg/entity/host"
	"sync"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"
)

// Group represents a set of runnable integrations that are located in
// the same integration configuration file, and thus share a common
// discovery mechanism configuration. It also does the following tasks:
// - parses integration output and forwards it
// - parses standard error and logs it
// - catches errors and logs them
// - manages the cancellation of tasks, as this should-be hot-reloaded
type Group struct {
	dSources     *databind.Sources
	integrations []integration.Definition
	emitter      emitter.Emitter
	// for testing purposes, allows defining which action to take when an execution
	// error is received. If unset, it will be runner.logErrors
	handleErrorsProvide  func() runnerErrorHandler
	cmdReqHandle         cmdrequest.HandleFn
	configHandle         configrequest.HandleFn
	terminateDefinitionQ chan string
	idLookup             host.IDLookup
}

type runnerErrorHandler func(ctx context.Context, errs <-chan error)

// NewGroup configures a Group instance that is provided by the passed LoadFn
// cfgPath is used for caching to be consumed by cmd-channel FF enabler.
func NewGroup(
	loadFn LoadFn,
	il integration.InstancesLookup,
	passthroughEnv []string,
	emitter emitter.Emitter,
	cmdReqHandle cmdrequest.HandleFn,
	configHandle configrequest.HandleFn,
	cfgPath string,
	terminateDefinitionQ chan string,
	idLookup host.IDLookup,
) (g Group, c FeaturesCache, err error) {

	g, c, err = loadFn(il, passthroughEnv, cfgPath, cmdReqHandle, configHandle, terminateDefinitionQ)
	if err != nil {
		return
	}

	g.emitter = emitter
	g.idLookup = idLookup

	return
}

// Run launches all the integrations to run in background. They can be cancelled with the
// provided context
func (g *Group) Run(ctx context.Context) (hasStartedAnyOHI bool) {
	for _, integr := range g.integrations {
		go NewRunner(integr, g.emitter, g.dSources, g.handleErrorsProvide, g.cmdReqHandle, g.configHandle, g.terminateDefinitionQ, g.idLookup).Run(ctx, nil, nil)
		hasStartedAnyOHI = true
	}

	return
}

// RunOnce will execute the group of integrations just one time.
func (g *Group) RunOnce(ctx context.Context) {

	wg := sync.WaitGroup{}
	for _, integrationDef := range g.integrations {
		integrationDef.Interval = 0
		wg.Add(1)
		go func(definition integration.Definition) {
			r := NewRunner(definition, g.emitter, g.dSources, g.handleErrorsProvide, g.cmdReqHandle, g.configHandle, g.terminateDefinitionQ, g.idLookup)
			r.Run(ctx, nil, nil)
			wg.Done()
		}(integrationDef)
	}

	wg.Wait()
}

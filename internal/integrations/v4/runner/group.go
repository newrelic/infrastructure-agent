// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"context"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/v3legacy"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
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
	discovery    *databind.Sources
	integrations []integration.Definition
	emitter      emitter.Emitter
	definitions  *v3legacy.DefinitionsRepo
	// for testing purposes, allows defining which action to take when an execution
	// error is received. If unset, it will be runner.logErrors
	handleErrorsProvide func() runnerErrorHandler
}

type runnerErrorHandler func(errs <-chan error)

// NewGroup configures a Group instance that is provided by the passed Loader
// cfgPath is used for caching to be consumed by cmd-channel FF enabler.
func NewGroup(
	loader Loader,
	dr integration.InstancesLookup,
	passthroughEnv []string,
	emitter emitter.Emitter,
	cfgPath string,
) (g Group, c FeaturesCache, err error) {

	g, c, err = loader(dr, passthroughEnv, cfgPath)
	if err != nil {
		return
	}

	g.emitter = emitter

	return
}

// Run launches all the integrations to run in background. They can be cancelled with the
// provided context
func (t *Group) Run(ctx context.Context) (hasStartedAnyOHI bool) {
	for _, integr := range t.integrations {
		r := NewRunner(integr, t.emitter, t.discovery)
		if t.handleErrorsProvide != nil {
			r.handleErrors = t.handleErrorsProvide()
		}

		go r.Run(ctx)
		hasStartedAnyOHI = true
	}

	return
}

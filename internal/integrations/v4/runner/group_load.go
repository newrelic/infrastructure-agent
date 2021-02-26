// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/cmdrequest"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/configrequest"
	config2 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
)

// LoadFn provides a basic, incomplete Group instance to be configured by the NewGroup function.
// InstancesLookup is only required to load v3 integrations from an external definitions folder.
type LoadFn func(dr integration.InstancesLookup, passthroughEnv []string, cfgPath string, cmdReqHandle cmdrequest.HandleFn, configHandle configrequest.HandleFn) (Group, FeaturesCache, error)

// NewLoadFn returns a function that provides partial Group holding provided configuration and
// features cache. Optionally agent and integration "features" can be provided to be able to load
// disabled integrations.
func NewLoadFn(cfg config2.YAML, agentAndCCFeatures *Features) LoadFn {
	return func(il integration.InstancesLookup, passthroughEnv []string, cfgPath string, cmdReqHandle cmdrequest.HandleFn, configHandle configrequest.HandleFn) (g Group, c FeaturesCache, err error) {
		dSources, err := cfg.Databind.DataSources()
		if err != nil {
			return
		}

		g = Group{
			dSources:     dSources,
			cmdReqHandle: cmdReqHandle,
			configHandle: configHandle,
		}
		c = make(FeaturesCache)

		for _, cfgEntry := range cfg.Integrations {
			var template []byte
			template, err = integration.LoadConfigTemplate(cfgEntry.TemplatePath, cfgEntry.Config)
			if err != nil {
				return
			}
			var i integration.Definition
			i, err = integration.NewDefinition(cfgEntry, il, passthroughEnv, template)
			if err != nil {
				return
			}

			if agentAndCCFeatures == nil {
				if cfgEntry.When.Feature == "" {
					// no features at all => run
					g.integrations = append(g.integrations, i)
				} else {
					// if feature only in OHI => cache for possible later usage via CC
					c[cfgEntry.When.Feature] = cfgPath
				}
				continue
			}

			// agent cfg or CC have feature, they decide:
			c[cfgEntry.When.Feature] = cfgPath
			if agentAndCCFeatures.IsOHIExecutable(cfgEntry.When.Feature) {
				g.integrations = append(g.integrations, i)
			}
		}

		return
	}
}

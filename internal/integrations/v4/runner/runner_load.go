// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

import (
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	config2 "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
)

// Loader provides a basic, incomplete Group instance to be configured by the
// NewGroup function. The DefinitionsRepo instance is only required to load
// v3 legacy integrations from an external definitions' folder.
type Loader func(dr integration.InstancesLookup, passthroughEnv []string, cfgPath string) (Group, FeaturesCache, error)

// NewLoader returns a partial Group that holds the configuration from the provided configuration.
// Optionally agent and OHI "features" can be provided to be able to load disabled OHIs.
func NewLoader(cfg config2.YAML, agentAndCCFeatures *Features) Loader {
	return func(dr integration.InstancesLookup, passthroughEnv []string, cfgPath string) (g Group, c FeaturesCache, err error) {
		discovery, err := databind.DataSources(&cfg.Databind)
		if err != nil {
			return
		}

		g = Group{
			discovery: discovery,
		}
		c = make(FeaturesCache)

		for _, cfgEntry := range cfg.Integrations {
			var template []byte
			template, err = integration.LoadConfigTemplate(cfgEntry.TemplatePath, cfgEntry.Config)
			if err != nil {
				return
			}
			var i integration.Definition
			i, err = integration.NewDefinition(cfgEntry, dr, passthroughEnv, template)
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

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package runner

// FeaturesCache stores per feature name it's config path.
type FeaturesCache map[string]string

// Update updates current cache merging from  provided one.
func (c FeaturesCache) Update(newFC FeaturesCache) {
	for f, cfgPath := range newFC {
		illog.WithField("cfgPath", cfgPath).Debug("Updated runner.")
		c[f] = cfgPath
	}
}

// Features carries the agent config features and a cmd-channel feature, to be able to determine
// a future OHI execution.
type Features struct {
	cmdFeat    *CmdFF // could be empty
	agentFeats map[string]bool
}

// CmdFF DTO storing a request from the cmd-channel to enable or disable an integration feature.
type CmdFF struct {
	Name    string // feature flag name
	Enabled bool
}

// IsEnabledOHI returns if enabled or nil in case it's unknown.
func (c *CmdFF) IsEnabledOHI(ff string) *bool {
	enabled := c.Name == ff && c.Enabled

	return &enabled
}

// NewFeatures creates a features object bundling features provided by agent and cmd-channel FF.
// If no matching feature is found nil is returned.
// Formats btw CC FF and config files:
// - Cmd-Channel FF:
//   | docker_enabled
// - Agent yaml:
//   | features:
//   |  docker_enabled: true
// - OHI yaml:
//   | - name: nri-docker
//   |   when:
//   |     feature: docker_enabled
func NewFeatures(fromAgent map[string]bool, cmdFF *CmdFF) *Features {
	featuresFromA := make(map[string]bool)
	for ff, enabled := range fromAgent {
		featuresFromA[ff] = enabled
	}

	return &Features{
		agentFeats: featuresFromA,
		cmdFeat:    cmdFF,
	}
}

// IsOHIExecutable determines the execution of an OHI given the features from config fields (agent & OHI)
// and cmd-channel. Rules:
// a. !ohi FF                   -> executed
// b.  ohi FF +  agent FF       -> agent determines
// c.  ohi FF + !agent FF + !CC -> not executed
// d.  ohi FF + !agent FF +  CC -> cmd-channel determines
func (f *Features) IsOHIExecutable(featureFromOHICfg string) bool {
	// a.
	if featureFromOHICfg == "" {
		return true
	}

	// b.
	if agentFF, ok := f.agentFeats[featureFromOHICfg]; ok {
		return agentFF
	}

	// c.
	if f.cmdFeat == nil {
		return false
	}

	// d.
	if isEnabled := f.cmdFeat.IsEnabledOHI(featureFromOHICfg); isEnabled != nil {
		return *isEnabled
	}

	return false
}

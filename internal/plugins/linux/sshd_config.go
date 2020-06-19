// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var sshdlog = log.WithPlugin("SshdConfig")

type SshdConfigPlugin struct {
	agent.PluginCommon
	frequency time.Duration
}

func NewSshdConfigPlugin(id ids.PluginID, ctx agent.AgentContext) *SshdConfigPlugin {
	cfg := ctx.Config()
	return &SshdConfigPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		frequency: config.ValidateConfigFrequencySetting(
			cfg.SshdConfigRefreshSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_SSHD_CONFIG_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

type SshdConfigValue struct {
	Key   string `json:"id"`
	Value string `json:"value"`
}

func (self SshdConfigValue) SortKey() string {
	return self.Key
}

var SshdConfigConfigRegexp *regexp.Regexp
var SshdConfigProperties = map[string]bool{
	"PermitRootLogin":                 true,
	"PermitEmptyPasswords":            true,
	"PasswordAuthentication":          true,
	"ChallengeResponseAuthentication": true,
}

func init() {
	pattern := `(?m)^(%s)`
	parts := []string{}
	for key := range SshdConfigProperties {
		// Optional leading space, the key, some space, the value, word boundary
		// e.g. PermitRootLogin( )(yes)\b
		parts = append(parts, fmt.Sprintf(`(\s*)%s\s+([\w-]+)\b`, key))
	}
	pattern = fmt.Sprintf(pattern, strings.Join(parts, "|"))
	SshdConfigConfigRegexp = regexp.MustCompile(pattern)
}

func parseSshdConfig(configText string) (values map[string]string, err error) {
	values = make(map[string]string)
	lines := SshdConfigConfigRegexp.FindAllString(configText, -1)
	for _, line := range lines {
		fields := strings.Fields(line)

		if len(fields) != 2 {
			sshdlog.WithField("line", line).Warn("invalid line detected in sshd config")
			continue
		}

		key, value := fields[0], fields[1]
		if _, wanted := SshdConfigProperties[key]; wanted {
			values[key] = value
		} else {
			sshdlog.WithField("key", key).Warn("captured unknown config key from ssh config")
			continue
		}
	}

	return
}

func convertSshValuesToPluginData(configValues map[string]string) (dataset agent.PluginInventoryDataset) {
	for key, value := range configValues {
		dataset = append(dataset, SshdConfigValue{key, value})
	}
	return
}

func (self *SshdConfigPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		sshdlog.Debug("Disabled.")
		return
	}

	refreshTimer := time.NewTicker(self.frequency)
	for {
		configBuf, err := ioutil.ReadFile(helpers.HostEtc("/ssh/sshd_config"))
		if err != nil {
			sshdlog.WithError(err).Error("reading sshd config file")
			self.Unregister()
			return
		}
		config, err := parseSshdConfig(string(configBuf))
		if err != nil {
			sshdlog.WithError(err).Error("parsing sshd config file")
		} else {
			self.EmitInventory(convertSshValuesToPluginData(config), self.Context.AgentIdentifier())
		}
		<-refreshTimer.C
	}
}

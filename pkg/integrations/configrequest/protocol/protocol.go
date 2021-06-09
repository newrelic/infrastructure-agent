// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package protocol

import (
	"encoding/json"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/config"
)

type discriminator struct {
	Version string `json:"config_protocol_version"`
}

type Builder interface {
	Build() (ConfigProtocol, error)
}

type builder struct {
	content []byte
	fn      func() ConfigProtocol
}

func (builder *builder) Build() (ConfigProtocol, error) {
	var cfgProtocol = builder.fn()
	var err error
	if err = json.Unmarshal(builder.content, cfgProtocol); err != nil {
		return cfgProtocol, err
	}
	if err = cfgProtocol.validate(); err != nil {
		return cfgProtocol, err
	}
	return cfgProtocol, nil
}

var defaultBuilderFn = func() ConfigProtocol {
	return &v1{}
}

var cfgProtocolVersions = map[string]func() ConfigProtocol{
	"1": func() ConfigProtocol { return &v1{} },
}

type ConfigProtocol interface {
	Name() string
	Version() int
	Integrations() []config.ConfigEntry
	GetConfig() databind.YAMLConfig
	validate() error
}

func GetConfigProtocolBuilder(content []byte) Builder {
	val := &discriminator{}
	if err := json.Unmarshal(content, val); err != nil || val.Version == "" {
		return nil
	}
	builderFn := cfgProtocolVersions[val.Version]
	if builderFn == nil {
		builderFn = defaultBuilderFn
	}
	return &builder{
		content: content,
		fn:      builderFn,
	}
}

type Context struct {
	ParentName string
	ConfigName string
}

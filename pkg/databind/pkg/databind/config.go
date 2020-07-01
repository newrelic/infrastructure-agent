// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"errors"
	"fmt"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/command"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/docker"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/fargate"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/secrets"
	"gopkg.in/yaml.v2"
)

const (
	defaultDiscoveryTTL = time.Minute
	defaultVariablesTTL = time.Hour
)

type YAMLConfig struct {
	Variables map[string]varEntry `yaml:"variables,omitempty"` // key: variable name
	Discovery struct {
		TTL     string               `yaml:"ttl,omitempty"`
		Docker  *discovery.Container `yaml:"docker,omitempty"`
		Fargate *discovery.Container `yaml:"fargate,omitempty"`
		Command *discovery.Command   `yaml:"command,omitempty"`
	} `yaml:"discovery"`
}

func (y *YAMLConfig) Enabled() bool {
	return len(y.Variables) > 0 ||
		y.Discovery.Docker != nil ||
		y.Discovery.Fargate != nil ||
		y.Discovery.Command != nil
}

type varEntry struct {
	TTL   string         `yaml:"ttl,omitempty"`
	KMS   *secrets.KMS   `yaml:"aws-kms,omitempty"`
	Vault *secrets.Vault `yaml:"vault,omitempty"`
}

// LoadYaml builds a set of data binding Sources from a YAML file
func LoadYAML(bytes []byte) (*Sources, error) {
	// Load raw yaml
	dc := YAMLConfig{}
	if err := yaml.Unmarshal(bytes, &dc); err != nil {
		return nil, err
	}
	return DataSources(&dc)
}

// DataSources builds a set of data binding sources from a YAMLConfig instance
func DataSources(dc *YAMLConfig) (*Sources, error) {
	if err := dc.validate(); err != nil {
		return nil, fmt.Errorf("error parsing YAML configuration: %s", err)
	}

	ttl, err := duration(dc.Discovery.TTL, defaultDiscoveryTTL)
	if err != nil {
		return nil, err
	}
	cfg := Sources{clock: time.Now, variables: map[string]*gatherer{}}
	cfg.discoverer, err = selectDiscoverer(ttl, dc)
	if err != nil {
		return nil, err
	}

	for name, vg := range dc.Variables {
		ttl, err := duration(vg.TTL, defaultVariablesTTL)
		if err != nil {
			return nil, err
		}
		cfg.variables[name] = selectGatherer(ttl, &vg)
	}

	return &cfg, nil
}

// returns a duration in the formatted string. If the string is empty, returns def (default)
// if the format is wrong, returns the default and an error
func duration(fmt string, def time.Duration) (time.Duration, error) {
	if fmt != "" {
		duration, err := time.ParseDuration(fmt)
		if err != nil {
			return def, err
		}
		return duration, nil
	}
	return def, nil
}

func selectDiscoverer(ttl time.Duration, dc *YAMLConfig) (*discoverer, error) {
	if dc.Discovery.Fargate != nil {
		fetch, err := fargate.Discoverer(*dc.Discovery.Fargate)
		if err != nil {
			return nil, err
		}
		return &discoverer{
			cache: cachedEntry{ttl: ttl},
			fetch: fetch,
		}, nil
	} else if dc.Discovery.Docker != nil {
		fetch, err := docker.Discoverer(*dc.Discovery.Docker)
		if err != nil {
			return nil, err
		}
		return &discoverer{
			cache: cachedEntry{ttl: ttl},
			fetch: fetch,
		}, nil
	} else if dc.Discovery.Command != nil {
		fetch, err := command.Discoverer(*dc.Discovery.Command)
		if err != nil {
			return nil, err
		}
		return &discoverer{
			cache: cachedEntry{ttl: ttl},
			fetch: fetch,
		}, nil
	}
	return nil, nil
}

func selectGatherer(ttl time.Duration, vg *varEntry) *gatherer {
	if vg.KMS != nil {
		return &gatherer{
			cache: cachedEntry{ttl: ttl},
			fetch: secrets.KMSGatherer(vg.KMS),
		}
	} else if vg.Vault != nil {
		return &gatherer{
			cache: cachedEntry{ttl: ttl},
			fetch: secrets.VaultGatherer(vg.Vault),
		}
	}
	// should never reach here as long as "varEntry.validate()" does its job
	// anyway, returning an error gatherer to avoid unexpected panics
	return &gatherer{fetch: func() (interface{}, error) {
		return "", errors.New("missing variable data source")
	}}
}

func (y *YAMLConfig) validate() error {
	sections := 0
	if y.Discovery.Docker != nil {
		sections++
		if err := y.Discovery.Docker.Validate(); err != nil {
			return err
		}
	}
	if y.Discovery.Fargate != nil {
		sections++
		if err := y.Discovery.Fargate.Validate(); err != nil {
			return err
		}
	}

	if y.Discovery.Command != nil {
		sections++
		if err := y.Discovery.Command.Validate(); err != nil {
			return err
		}
	}

	if sections > 1 {
		return errors.New("only one discovery source allowed")
	}

	names := map[string]struct{}{}
	for name, vg := range y.Variables {
		if _, ok := names[name]; ok {
			return fmt.Errorf("duplicate variable name %q", names)
		}
		names[name] = struct{}{}
		if err := vg.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (ve *varEntry) validate() error {
	sections := 0
	if ve.KMS != nil {
		sections++
		if err := ve.KMS.Validate(); err != nil {
			return err
		}
	}
	if ve.Vault != nil {
		sections++
		if err := ve.Vault.Validate(); err != nil {
			return err
		}
	}
	if sections == 0 {
		return errors.New("you should specify one source to gather the variable: aws-kms or vault")
	}
	if sections > 1 {
		return errors.New("you can't specify more than one source into a single variable. Use another variable")
	}
	return nil
}

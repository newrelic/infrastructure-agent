// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package databind

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/command"
	yaml "gopkg.in/yaml.v2"

	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/docker"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/discovery/fargate"
	"github.com/newrelic/infrastructure-agent/pkg/databind/internal/secrets"
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
	TTL         string               `yaml:"ttl,omitempty"`
	KMS         *secrets.KMS         `yaml:"aws-kms,omitempty"`
	Vault       *secrets.Vault       `yaml:"vault,omitempty"`
	CyberArkCLI *secrets.CyberArkCLI `yaml:"cyberark-cli,omitempty"`
	CyberArkAPI *secrets.CyberArkAPI `yaml:"cyberark-api,omitempty"`
	Obfuscated  *secrets.Obfuscated  `yaml:"obfuscated,omitempty"`
}

// LoadYaml builds a set of data binding Sources from a YAML file
func LoadYAML(bytes []byte) (*Sources, error) {
	// Load raw yaml
	dc := YAMLConfig{}
	if err := yaml.Unmarshal(bytes, &dc); err != nil {
		return nil, err
	}

	return dc.DataSources()
}

// DataSources builds a set of data binding sources for the YAMLConfig instance.
func (dc *YAMLConfig) DataSources() (*Sources, error) {
	if err := dc.validate(); err != nil {
		return nil, fmt.Errorf("error parsing YAML configuration: %s", err)
	}

	ttl, err := duration(dc.Discovery.TTL, defaultDiscoveryTTL)
	if err != nil {
		return nil, err
	}

	cfg := Sources{
		clock:     time.Now,
		variables: map[string]*gatherer{},
	}
	cfg.discoverer, err = dc.selectDiscoverer(ttl)
	if err != nil {
		return nil, err
	}

	for vName, vEntry := range dc.Variables {
		ttl, err := duration(vEntry.TTL, defaultVariablesTTL)
		if err != nil {
			return nil, err
		}
		cfg.variables[vName] = vEntry.selectGatherer(ttl)
	}

	return &cfg, nil
}

// returns a duration in the formatted string. If the string is empty, returns def (default)
// if the format is wrong, returns the default and an error
func duration(fmt string, def time.Duration) (time.Duration, error) {
	if fmt == "" {
		return def, nil
	}

	duration, err := time.ParseDuration(fmt)
	if err != nil {
		return def, err
	}

	return duration, nil
}

func (dc *YAMLConfig) selectDiscoverer(ttl time.Duration) (*discoverer, error) {
	if dc.Discovery.Fargate != nil {
		fetch, err := fargate.Discoverer(*dc.Discovery.Fargate)
		return &discoverer{
			cache: cachedEntry{ttl: ttl},
			fetch: fetch,
		}, err

	} else if dc.Discovery.Docker != nil {
		fetch, err := docker.Discoverer(*dc.Discovery.Docker)
		return &discoverer{
			cache: cachedEntry{ttl: ttl},
			fetch: fetch,
		}, err

	} else if dc.Discovery.Command != nil {
		fetch, err := command.Discoverer(*dc.Discovery.Command)
		return &discoverer{
			cache: cachedEntry{ttl: ttl},
			fetch: fetch,
		}, err

	}
	return nil, nil
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
	for vName, vEntry := range y.Variables {
		if _, ok := names[vName]; ok {
			return fmt.Errorf("duplicate variable name %q", names)
		}
		vEntry.expand()

		names[vName] = struct{}{}
		if err := vEntry.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (v *varEntry) expand() {
	if v.Obfuscated != nil {
		expandEnvVars(v.Obfuscated)
	}
}

func (v *varEntry) validate() error {
	sections := 0
	if v.KMS != nil {
		sections++
		if err := v.KMS.Validate(); err != nil {
			return err
		}
	}
	if v.Vault != nil {
		sections++
		if err := v.Vault.Validate(); err != nil {
			return err
		}
	}
	if v.CyberArkCLI != nil {
		sections++
		if err := v.CyberArkCLI.Validate(); err != nil {
			return err
		}
	}
	if v.CyberArkAPI != nil {
		sections++
		if err := v.CyberArkAPI.Validate(); err != nil {
			return err
		}
	}
	if v.Obfuscated != nil {
		sections++
		if err := v.Obfuscated.Validate(); err != nil {
			return err
		}
	}
	if sections == 0 {
		return errors.New("you should specify one source to gather the variable: aws-kms or vault or cyberark-cli")
	}
	if sections > 1 {
		return errors.New("you can't specify more than one source into a single variable. Use another variable")
	}
	return nil
}

func (v *varEntry) selectGatherer(ttl time.Duration) *gatherer {
	if v.KMS != nil {
		return &gatherer{
			cache: cachedEntry{ttl: ttl},
			fetch: secrets.KMSGatherer(v.KMS),
		}

	} else if v.Vault != nil {
		return &gatherer{
			cache: cachedEntry{ttl: ttl},
			fetch: secrets.VaultGatherer(v.Vault),
		}

	} else if v.CyberArkCLI != nil {
		return &gatherer{
			cache: cachedEntry{ttl: ttl},
			fetch: secrets.CyberArkCLIGatherer(v.CyberArkCLI),
		}

	} else if v.CyberArkAPI != nil {
		return &gatherer{
			cache: cachedEntry{ttl: ttl},
			fetch: secrets.CyberArkAPIGatherer(v.CyberArkAPI),
		}
	} else if v.Obfuscated != nil {
		return &gatherer{
			cache: cachedEntry{ttl: ttl},
			fetch: secrets.ObfuscateGatherer(v.Obfuscated),
		}
	}

	// should never reach here as long as "varEntry.validate()" does its job
	// anyway, returning an error gatherer to avoid unexpected panics
	return &gatherer{fetch: func() (interface{}, error) {
		return "", errors.New("missing variable data source")
	}}
}

func expandEnvVars(obj interface{}) {
	e := reflect.ValueOf(obj).Elem()

	for i := 0; i < e.NumField(); i++ {
		value := e.Field(i).Interface()

		valueStr, ok := value.(string)
		if !ok {
			continue
		}

		if ok := strings.HasPrefix(valueStr, "$"); !ok {
			continue
		}

		if envVar, ok := os.LookupEnv(valueStr[1:]); ok {
			e.Field(i).SetString(envVar)
		}
	}
}

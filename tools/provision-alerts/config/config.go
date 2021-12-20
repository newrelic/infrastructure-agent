package config

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
	"text/template"
)

type Config struct {
	Policies PolicyConfigs `yaml:"policies"`
}

type PolicyConfigs []PolicyConfig

type PolicyConfig struct {
	Name               string           `yaml:"name"`
	IncidentPreference string           `yaml:"incident_preference"`
	NrqlTemplates      NrqlTemplates    `yaml:"nrql_templates"`
	Conditions         ConditionConfigs `yaml:"conditions"`
	Channels           []int            `yaml:"channels"`
}

type NrqlTemplates []NrqlTemplate

type NrqlTemplate struct {
	Name string `yaml:"name"`
	NRQL string `yaml:"nrql"`
}

type ConditionConfigs []ConditionConfig

type ConditionConfig struct {
	Name         string  `yaml:"name"`
	Metric       string  `yaml:"metric"`
	Sample       string  `yaml:"sample"`
	Duration     int     `yaml:"duration"`
	Threshold    float64 `yaml:"threshold"`
	Operator     string  `yaml:"operator"`
	TemplateName string  `yaml:"template_name"`
	NRQL         string
}

func ParseConfig(raw []byte) (Config, error) {
	cfg := Config{}
	err := yaml.Unmarshal(raw, &cfg)

	return cfg, err
}

type templates map[string]string

func FulfillConfig(config Config, displayNameCurrent, displayNamePrevious string) (Config, error) {

	for policyIdx, policy := range config.Policies {
		tmpls := parseTemplates(policy.NrqlTemplates)
		for conditionIdx, cc := range policy.Conditions {
			if _, ok := tmpls[cc.TemplateName]; !ok {
				return Config{}, fmt.Errorf("cannot find %s template", cc.TemplateName)
			}
			fulfilledCc, err := FulfillConditionConfig(cc, tmpls[cc.TemplateName], displayNameCurrent, displayNamePrevious)
			if err != nil {
				return Config{}, err
			}
			config.Policies[policyIdx].Conditions[conditionIdx] = fulfilledCc
		}
		// Policies have a max of 700 conditions. To create one policy per OS/ARCH
		// we rename policy to include it from displayName.
		displayNameParts := strings.Split(displayNameCurrent, ":")
		if len(displayNameParts) > 2 {
			os := displayNameParts[len(displayNameParts)-1]
			arch := displayNameParts[len(displayNameParts)-2]
			config.Policies[policyIdx].Name += " / " + os + " / " + arch
		} else {
			config.Policies[policyIdx].Name += " / " + displayNameCurrent
		}
	}

	return config, nil
}

func parseTemplates(tmplsConfig NrqlTemplates) templates {
	tmpls := make(templates)
	for _, t := range tmplsConfig {
		tmpls[t.Name] = t.NRQL
	}
	return tmpls
}

func FulfillConditionConfig(conditionConfig ConditionConfig, nrqlTemplate string, displayNameCurrent, displayNamePrevious string) (ConditionConfig, error) {

	tmpl, err := template.New("nrql").Parse(nrqlTemplate)

	if err != nil {
		return ConditionConfig{}, err
	}

	type fields struct {
		Metric              string
		Sample              string
		DisplayNameCurrent  string
		DisplayNamePrevious string
	}

	var nrql bytes.Buffer
	err = tmpl.Execute(&nrql, fields{
		Metric:              conditionConfig.Metric,
		Sample:              conditionConfig.Sample,
		DisplayNameCurrent:  displayNameCurrent,
		DisplayNamePrevious: displayNamePrevious,
	})

	if err != nil {
		return ConditionConfig{}, err
	}

	conditionConfig.NRQL = nrql.String()

	return conditionConfig, nil
}

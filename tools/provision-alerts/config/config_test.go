package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ParsePolicy(t *testing.T) {

	cfgYAML := `
policies:
  - name: "policy name"
    incident_preference: "something"
    channels: [3423432]
    nrql_templates:
      - name: Generic metric comparator
        nrql:
          SELECT abs(
            filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNameCurrent }}')
            -
            filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNamePrevious }}')
            )
          FROM {{ .Sample }}
          WHERE displayName IN ('{{ .DisplayNameCurrent }}','{{ .DisplayNamePrevious }}')

      - name: Static example template
        nrql: "SELECT COUNT(*) FROM Log WHERE message LIKE '%error%'"
    conditions:
      - name: System / Cpu Percent
        metric: cpuPercent
        sample: SystemSample
        threshold: 3
        duration: 10
        operator: "above"
        template_name: "Generic metric comparator"

      - name: System / Cpu Percent
        threshold: 3
        duration: 10
        operator: "above"
        template_name: "Static example template"

`

	expected := Config{
		Policies: PolicyConfigs{
			{
				Name:               "policy name",
				IncidentPreference: "something",
				Channels:           []int{3423432},
				NrqlTemplates: NrqlTemplates{
					{
						Name: "Generic metric comparator",
						NRQL: "SELECT abs( filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNameCurrent }}') - filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNamePrevious }}') ) FROM {{ .Sample }} WHERE displayName IN ('{{ .DisplayNameCurrent }}','{{ .DisplayNamePrevious }}')",
					},
					{
						Name: "Static example template",
						NRQL: "SELECT COUNT(*) FROM Log WHERE message LIKE '%error%'",
					},
				},
				Conditions: ConditionConfigs{
					{
						Name:         "System / Cpu Percent",
						Metric:       "cpuPercent",
						Sample:       "SystemSample",
						Threshold:    3,
						Duration:     10,
						Operator:     "above",
						TemplateName: "Generic metric comparator",
					},
					{
						Name:         "System / Cpu Percent",
						Threshold:    3,
						Duration:     10,
						Operator:     "above",
						TemplateName: "Static example template",
					},
				},
			},
		},
	}

	actual, err := ParseConfig([]byte(cfgYAML))

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

}

func TestFullFillConditionConfig(t *testing.T) {

	condition := ConditionConfig{
		Name:         "System / Cpu Percent",
		Metric:       "cpuPercent",
		Sample:       "SystemSample",
		Threshold:    3,
		Duration:     10,
		Operator:     "above",
		TemplateName: "Generic metric comparator",
	}

	template := NrqlTemplate{
		Name: "Generic metric comparator",
		NRQL: "SELECT abs( filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNameCurrent }}') - filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNamePrevious }}') ) FROM {{ .Sample }} WHERE displayName IN ('{{ .DisplayNameCurrent }}','{{ .DisplayNamePrevious }}')",
	}

	displayNameCurrent := "current"
	displayNamePrevious := "previous"

	expectedCondition := ConditionConfig{
		Name:         "System / Cpu Percent",
		Metric:       "cpuPercent",
		Sample:       "SystemSample",
		Threshold:    3,
		Duration:     10,
		Operator:     "above",
		TemplateName: "Generic metric comparator",
		NRQL:         "SELECT abs( filter(average(cpuPercent), WHERE displayName like 'current') - filter(average(cpuPercent), WHERE displayName like 'previous') ) FROM SystemSample WHERE displayName IN ('current','previous')",
	}

	actualCondition, err := FulfillConditionConfig(condition, template.NRQL, displayNameCurrent, displayNamePrevious)

	assert.NoError(t, err)
	assert.Equal(t, expectedCondition, actualCondition)
}

func Test_FulfillConfig(t *testing.T) {

	config := Config{
		Policies: PolicyConfigs{
			{
				Name:               "policy name",
				IncidentPreference: "something",
				Channels:           []int{3423432},
				NrqlTemplates: NrqlTemplates{
					{
						Name: "Generic metric comparator",
						NRQL: "SELECT abs( filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNameCurrent }}') - filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNamePrevious }}') ) FROM {{ .Sample }} WHERE displayName IN ('{{ .DisplayNameCurrent }}','{{ .DisplayNamePrevious }}')",
					},
					{
						Name: "Static example template",
						NRQL: "SELECT COUNT(*) FROM Log WHERE message LIKE '%error%'",
					},
				},
				Conditions: ConditionConfigs{
					{
						Name:         "System / Cpu Percent",
						Metric:       "cpuPercent",
						Sample:       "SystemSample",
						Threshold:    3,
						Duration:     10,
						Operator:     "above",
						TemplateName: "Generic metric comparator",
					},
					{
						Name:         "System / Cpu Percent",
						Threshold:    3,
						Duration:     10,
						Operator:     "above",
						TemplateName: "Static example template",
					},
				},
			},
		},
	}

	displayNameCurrent := "some display name current"
	displayNamePrevious := "some display name previous"

	expectedConfig := Config{
		Policies: PolicyConfigs{
			{
				Name:               "policy name",
				IncidentPreference: "something",
				Channels:           []int{3423432},
				NrqlTemplates: NrqlTemplates{
					{
						Name: "Generic metric comparator",
						NRQL: "SELECT abs( filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNameCurrent }}') - filter(average({{ .Metric }}), WHERE displayName like '{{ .DisplayNamePrevious }}') ) FROM {{ .Sample }} WHERE displayName IN ('{{ .DisplayNameCurrent }}','{{ .DisplayNamePrevious }}')",
					},
					{
						Name: "Static example template",
						NRQL: "SELECT COUNT(*) FROM Log WHERE message LIKE '%error%'",
					},
				},
				Conditions: ConditionConfigs{
					{
						Name:         "System / Cpu Percent",
						Metric:       "cpuPercent",
						Sample:       "SystemSample",
						Threshold:    3,
						Duration:     10,
						Operator:     "above",
						TemplateName: "Generic metric comparator",
						NRQL:         "SELECT abs( filter(average(cpuPercent), WHERE displayName like 'some display name current') - filter(average(cpuPercent), WHERE displayName like 'some display name previous') ) FROM SystemSample WHERE displayName IN ('some display name current','some display name previous')",
					},
					{
						Name:         "System / Cpu Percent",
						Threshold:    3,
						Duration:     10,
						Operator:     "above",
						TemplateName: "Static example template",
						NRQL:         "SELECT COUNT(*) FROM Log WHERE message LIKE '%error%'",
					},
				},
			},
		},
	}

	actualConfig, err := FulfillConfig(config, displayNameCurrent, displayNamePrevious)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, actualConfig)
}

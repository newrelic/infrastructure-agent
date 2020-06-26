// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sampler_test

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var metricDimensionProcessName string = "process.name"
var metricDimensionProcessExecutable string = "process.executable"

func Test_EvaluatorChain_WithSingleRule(t *testing.T) {

	type testCase struct {
		name  string
		input interface{}
		rules map[string][]string
		want  bool
	}

	cases := []testCase{
		{
			name:  "ProcessName_IsLiteralMatch",
			input: metrics.ProcessSample{ProcessDisplayName: "java"},
			rules: map[string][]string{
				metricDimensionProcessName: {"java"},
			},
			want: true,
		},
		{
			name:  "ProcessName_IsLiteralNotMatch",
			input: metrics.ProcessSample{ProcessDisplayName: "java"},
			rules: map[string][]string{
				metricDimensionProcessName: {"test"},
			},
			want: false,
		},
		{
			name:  "ProcessName_IsRegexMatch",
			input: metrics.ProcessSample{ProcessDisplayName: "test.exe"},
			rules: map[string][]string{
				metricDimensionProcessName: {"regex ^test"},
			},
			want: true,
		},
		{
			name:  "ProcessName_IsRegexNotMatch",
			input: metrics.ProcessSample{ProcessDisplayName: "java.exe"},
			rules: map[string][]string{
				metricDimensionProcessName: {"regex ^test"},
			},
			want: false,
		},
		{
			name:  "ProcessCmdLine_IsLiteralMatch",
			input: &metrics.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"/bin/java"},
			},
			want: true,
		},
		{
			name:  "ProcessCmdLine_IsLiteralNotMatch",
			input: &metrics.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"/bin/test"},
			},
			want: false,
		},
		{
			name:  "ProcessCmdLine_IsRegexMatch",
			input: &metrics.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"regex ^/bin/java"},
			},
			want: true,
		},
		{
			name:  "ProcessCmdLine_IsRegexNotMatch",
			input: &metrics.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"regex ^/bin/local/java"},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ec := sampler.NewMatcherChain(tc.rules)
			assert.Len(t, ec.Matchers, len(tc.rules))
			assert.EqualValues(t, tc.want, ec.Evaluate(tc.input))
		})
	}
}

// Although these test cases are checking specifically for the 2 required dimensions/attributes
// it can be easily adapted for any other number of dimensions/attributes
func Test_Evaluator_WithTwoLiteralRules(t *testing.T) {
	// ProcessDisplayName maps to "process.name" and CmdLine maps to "process.executable"
	javaProcessSample := metrics.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"}

	type testCase struct {
		name  string
		input interface{}
		rules map[string][]string
		want  bool
	}

	cases := []testCase{
		{
			name:  "ProcessNameAndExecutableAreMatch",
			input: javaProcessSample,
			rules: map[string][]string{
				metricDimensionProcessName:       {"java"},
				metricDimensionProcessExecutable: {"test.jar"},
			},
			want: true,
		},
		{
			name:  "ProcessNameAndExecutableAreNotMatch",
			input: javaProcessSample,
			rules: map[string][]string{
				metricDimensionProcessName:       {"test"},
				metricDimensionProcessExecutable: {"/bin/test"},
			},
			want: false,
		},
		{
			name:  "ProcessNameIsMatchAndExecutableIsNotMatch",
			input: javaProcessSample,
			rules: map[string][]string{
				metricDimensionProcessName:       {"java"},
				metricDimensionProcessExecutable: {"/etc/alternatives/java"},
			},
			want: true,
		},
		{
			// this test case is not very likely
			name:  "ProcessNameIsNotMatchAndExecutableIsMatch",
			input: javaProcessSample,
			rules: map[string][]string{
				metricDimensionProcessName:       {"java-9"},
				metricDimensionProcessExecutable: {"/bin/java"},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ec := sampler.NewMatcherChain(tc.rules)
			assert.Len(t, ec.Matchers, len(tc.rules))
			assert.EqualValues(t, tc.want, ec.Evaluate(tc.input))
		})
	}
}

func Test_Evaluator_WithUnMappedFields(t *testing.T) {
	javaProcessSample := metrics.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"}

	type testCase struct {
		name  string
		input interface{}
		rules map[string][]string
		want  bool
	}

	cases := []testCase{
		{
			name:  "UnMappedField",
			input: javaProcessSample,
			rules: map[string][]string{
				"process.unmappedFiled": {"foobar"},
			},
			want: false,
		},
		{
			name:  "OneUnMappedFieldAndOneFieldIsMatch",
			input: javaProcessSample,
			rules: map[string][]string{
				"process.unmappedField": {"somevalue"},
				"process.name":          {"java"},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ec := sampler.NewMatcherChain(tc.rules)
			assert.Len(t, ec.Matchers, len(tc.rules))
			assert.EqualValues(t, tc.want, ec.Evaluate(tc.input))
		})
	}
}

// Test_Evaluator_WithNonProcessSamples tests that other sample types keep working as expected
func Test_Evaluator_WithNonProcessSamples(t *testing.T) {
	networkSample := network.NetworkSample{InterfaceName: "eth0"}
	systemSample := metrics.SystemSample{
		CPUSample: &metrics.CPUSample{
			CPUPercent: 50,
		},
	}
	storageSample := storage.BaseSample{
		Device: "/dev/sda1",
	}

	type testCase struct {
		name  string
		input interface{}
		rules map[string][]string
		want  bool
	}

	cases := []testCase{
		{
			name:  "NetworkSample",
			input: networkSample,
			rules: map[string][]string{
				"process.name": {"foobar"},
			},
			want: true,
		},
		{
			name:  "SystemSample",
			input: systemSample,
			rules: map[string][]string{
				"process.name": {"foobar"},
			},
			want: true,
		},
		{
			name:  "StorageSample",
			input: storageSample,
			rules: map[string][]string{
				"process.name": {"foobar"},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ec := sampler.NewMatcherChain(tc.rules)
			assert.Len(t, ec.Matchers, len(tc.rules))
			assert.EqualValues(t, tc.want, ec.Evaluate(tc.input))
		})
	}
}

func Test_EvaluatorChain_WithMultipleRuleAttribute(t *testing.T) {

	type testCase struct {
		name  string
		input []interface{}
		rules map[string][]string
		want  []bool
	}

	cases := []testCase{
		{
			/*
				matchers:
				   process.name:
				     - java
				     - test
			*/
			name: "ProcessName_IsLiteralMatch",
			input: []interface{}{
				metrics.ProcessSample{ProcessDisplayName: "java"},
				metrics.ProcessSample{ProcessDisplayName: "test"},
			},
			rules: map[string][]string{
				metricDimensionProcessName: {
					"java",
					"test",
				},
			},
			want: []bool{true, true},
		},
		{
			/*
				matchers:
				  process.name:
				    - newrelic
			*/
			name: "ProcessName_IsLiteralNotMatch",
			input: []interface{}{
				metrics.ProcessSample{ProcessDisplayName: "java"},
				metrics.ProcessSample{ProcessDisplayName: "test"},
			},
			rules: map[string][]string{
				metricDimensionProcessName: {"newrelic"},
			},
			want: []bool{false, false},
		},
		{
			/*
				matchers:
				  process.name:
				    - regex ^java
			*/
			name: "ProcessName_IsRegexMatch",
			input: []interface{}{
				metrics.ProcessSample{ProcessDisplayName: "java.exe"},
				metrics.ProcessSample{ProcessDisplayName: "java-9.exe"},
			},
			rules: map[string][]string{
				metricDimensionProcessName: {"regex ^java"},
			},
			want: []bool{true, true},
		},
		{
			/*
				matchers:
				  process.name:
				    - regex ^test
			*/
			name: "ProcessName_IsRegexNotMatch",
			input: []interface{}{
				metrics.ProcessSample{ProcessDisplayName: "java.exe"},
				metrics.ProcessSample{ProcessDisplayName: "java-9.exe"},
			},
			rules: map[string][]string{
				metricDimensionProcessName: {"regex ^test"},
			},
			want: []bool{false, false},
		},
		{
			/*
				matchers:
				  process.executable:
				    - /bin/java
				    - /bin/local/java-9
			*/
			name: "ProcessExecutable_IsLiteralMatch",
			input: []interface{}{
				metrics.ProcessSample{CmdLine: "/bin/java"},
				metrics.ProcessSample{CmdLine: "/bin/local/java-9"},
			},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {
					"/bin/java",
					"/bin/local/java-9",
				},
			},
			want: []bool{true, true},
		},
		{
			/*
				matchers:
				  process.executable:
				    - /bin/test
				    - /bin/some-test
			*/
			name: "ProcessExecutalbe_IsLiteralNotMatch",
			input: []interface{}{
				metrics.ProcessSample{CmdLine: "/bin/java"},
				metrics.ProcessSample{CmdLine: "/bin/local/java-9.exe"},
			},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {
					"/bin/test",
					"/bin/some-test",
				},
			},
			want: []bool{false, false},
		},
		{
			/*
				matchers:
				  process.executable:
				    - regex ^/bin/java
				    - regex ^/bin/local/
			*/
			name: "ProcessExecutable_IsRegexMatch",
			input: []interface{}{
				metrics.ProcessSample{CmdLine: "/bin/java"},
				metrics.ProcessSample{CmdLine: "/bin/local/test"},
			},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {
					"regex ^/bin/java",
					"regex ^/bin/local/",
				},
			},
			want: []bool{true, true},
		},
		{
			/*
				matchers:
				  process.executable:
				    - regex ^/bin/test
				    - regex ^/bin/local/java
			*/
			name: "ProcessExecutable_IsRegexNotMatch",
			input: []interface{}{
				metrics.ProcessSample{CmdLine: "/bin/java"},
				metrics.ProcessSample{CmdLine: "/bin/local/test"},
			},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {
					"regex ^/bin/test",
					"regex ^/bin/local/java",
				},
			},
			want: []bool{false, false},
		},
		{
			name: "AllTogetherNow",
			input: []interface{}{
				metrics.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"},
				metrics.ProcessSample{ProcessDisplayName: "test", CmdLine: "/bin/local/test"},
				metrics.ProcessSample{ProcessDisplayName: "newrelic", CmdLine: "/bin/newrelic-infra"},
				metrics.ProcessSample{ProcessDisplayName: "kafka", CmdLine: "/bin/java"},
				metrics.ProcessSample{ProcessDisplayName: "important.exe", CmdLine: "c:\\program files\\my-app\\important.exe"},
				metrics.ProcessSample{ProcessDisplayName: "dhclient", CmdLine: "/sbin/dhclient"},
				metrics.ProcessSample{ProcessDisplayName: "dockerd", CmdLine: "/usr/bin/dockerd"},
			},
			rules: map[string][]string{
				metricDimensionProcessName: {
					"java",
					"regex ^kafka",
					"important.exe",
				},
				metricDimensionProcessExecutable: {
					"regex kafka",
					"regex ^/bin/local/",
					"regex ^/sbin",
				},
			},
			want: []bool{true, true, false, true, true, true, false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ec := sampler.NewMatcherChain(tc.rules)
			assert.Len(t, ec.Matchers, len(tc.rules))

			for i, input := range tc.input {
				assert.Equal(t, tc.want[i], ec.Evaluate(input), "input index: %v", i)
			}
		})
	}
}

func Test_EvaluatorChain_RulesWithQuotesAndSpaces(t *testing.T) {
	inputs := []interface{}{
		metrics.ProcessSample{ProcessDisplayName: "java"},
		metrics.ProcessSample{ProcessDisplayName: "test"},
	}

	rules := map[string][]string{
		metricDimensionProcessName: {
			"\" test\"",
			"regex \"^java\"",
		},
	}

	ec := sampler.NewMatcherChain(rules)
	for _, i := range inputs {
		assert.Equal(t, true, ec.Evaluate(i))
	}
}

func Test_EvaluatorChain_LogTraceMatcher(t *testing.T) {
	trace.EnableOn([]string{trace.METRIC_MATCHER.String()})
	log.SetOutput(ioutil.Discard) // discard logs so not to break race tests
	log.SetLevel(logrus.TraceLevel)
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	javaProcessSample := metrics.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"}

	rule := config.IncludeMetricsMap{"process.name": {"java"}}
	ec := sampler.NewMatcherChain(rule)

	assert.Len(t, ec.Matchers, len(rule))
	assert.EqualValues(t, true, ec.Evaluate(javaProcessSample))

	require.NotEmpty(t, hook.Entries)
	entry := hook.LastEntry()
	assert.Equal(t, "[metric.match] 'java' matches expression 'ProcessDisplayName' >> 'java': true", entry.Message)
	assert.Equal(t, logrus.TraceLevel, entry.Level)
}

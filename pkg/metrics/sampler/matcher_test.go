// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sampler_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	testFF "github.com/newrelic/infrastructure-agent/internal/feature_flags/test"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/metrics"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/network"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/sampler"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/storage"
	"github.com/newrelic/infrastructure-agent/pkg/metrics/types"
	fixture "github.com/newrelic/infrastructure-agent/test/fixture/sample"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

var metricDimensionProcessName = "process.name"
var metricDimensionProcessExecutable = "process.executable"

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
			input: types.ProcessSample{ProcessDisplayName: "java"},
			rules: map[string][]string{
				metricDimensionProcessName: {"java"},
			},
			want: true,
		},
		{
			name:  "ProcessName_IsLiteralNotMatch",
			input: types.ProcessSample{ProcessDisplayName: "java"},
			rules: map[string][]string{
				metricDimensionProcessName: {"test"},
			},
			want: false,
		},
		{
			name:  "FlatProcessName_IsLiteralMatch",
			input: types.FlatProcessSample{"processDisplayName": "java"},
			rules: map[string][]string{
				metricDimensionProcessName: {"java"},
			},
			want: true,
		},
		{
			name:  "FlatProcessName_IsLiteralNotMatch",
			input: types.FlatProcessSample{"processDisplayName": "java"},
			rules: map[string][]string{
				metricDimensionProcessName: {"test"},
			},
			want: false,
		},
		{
			name:  "ProcessName_IsRegexMatch",
			input: types.ProcessSample{ProcessDisplayName: "test.exe"},
			rules: map[string][]string{
				metricDimensionProcessName: {"regex ^test"},
			},
			want: true,
		},
		{
			name:  "ProcessName_IsRegexNotMatch",
			input: types.ProcessSample{ProcessDisplayName: "java.exe"},
			rules: map[string][]string{
				metricDimensionProcessName: {"regex ^test"},
			},
			want: false,
		},
		{
			name:  "ProcessCmdLine_IsLiteralMatch",
			input: &types.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"/bin/java"},
			},
			want: true,
		},
		{
			name:  "ProcessCmdLine_IsLiteralNotMatch",
			input: &types.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"/bin/test"},
			},
			want: false,
		},
		{
			name:  "ProcessCmdLine_IsRegexMatch",
			input: &types.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"regex ^/bin/java"},
			},
			want: true,
		},
		{
			name:  "ProcessCmdLine_IsRegexNotMatch",
			input: &types.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"regex ^/bin/local/java"},
			},
			want: false,
		},
		{
			name:  "ProcessCmdLine_NotValidRegex",
			input: &types.ProcessSample{CmdLine: "/bin/java"},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"regex *"},
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
	javaProcessSample := types.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"}

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
		{
			name:  "ProcessNameAndExecutableAreMatchNotValidRegex",
			input: javaProcessSample,
			rules: map[string][]string{
				metricDimensionProcessExecutable: {"regex *"},
				metricDimensionProcessName:       {"java"},
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
	javaProcessSample := types.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"}

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
				types.ProcessSample{ProcessDisplayName: "java"},
				types.ProcessSample{ProcessDisplayName: "test"},
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
				types.ProcessSample{ProcessDisplayName: "java"},
				types.ProcessSample{ProcessDisplayName: "test"},
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
				types.ProcessSample{ProcessDisplayName: "java.exe"},
				types.ProcessSample{ProcessDisplayName: "java-9.exe"},
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
				types.ProcessSample{ProcessDisplayName: "java.exe"},
				types.ProcessSample{ProcessDisplayName: "java-9.exe"},
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
				types.ProcessSample{CmdLine: "/bin/java"},
				types.ProcessSample{CmdLine: "/bin/local/java-9"},
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
				types.ProcessSample{CmdLine: "/bin/java"},
				types.ProcessSample{CmdLine: "/bin/local/java-9.exe"},
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
				types.ProcessSample{CmdLine: "/bin/java"},
				types.ProcessSample{CmdLine: "/bin/local/test"},
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
				types.ProcessSample{CmdLine: "/bin/java"},
				types.ProcessSample{CmdLine: "/bin/local/test"},
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
				types.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"},
				types.ProcessSample{ProcessDisplayName: "test", CmdLine: "/bin/local/test"},
				types.ProcessSample{ProcessDisplayName: "newrelic", CmdLine: "/bin/newrelic-infra"},
				types.ProcessSample{ProcessDisplayName: "kafka", CmdLine: "/bin/java"},
				types.ProcessSample{ProcessDisplayName: "important.exe", CmdLine: "c:\\program files\\my-app\\important.exe"},
				types.ProcessSample{ProcessDisplayName: "dhclient", CmdLine: "/sbin/dhclient"},
				types.ProcessSample{ProcessDisplayName: "dockerd", CmdLine: "/usr/bin/dockerd"},
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
		{
			/*
				matchers:
				  process.executable:
				    - regex *					# bad regex
				    - regex .*
			*/
			name: "ProcessExecutable_NotValidRegex",
			input: []interface{}{
				types.ProcessSample{CmdLine: "/bin/java"},
				types.ProcessSample{CmdLine: "/bin/local/java"},
			},
			rules: map[string][]string{
				metricDimensionProcessExecutable: {
					"regex *",
					"/bin/java",
					"/bin/local/java",
				},
			},
			want: []bool{true, true},
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
		types.ProcessSample{ProcessDisplayName: "java"},
		types.ProcessSample{ProcessDisplayName: "test"},
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
	log.SetOutput(ioutil.Discard) // discard logs so not to break race tests
	log.SetLevel(logrus.TraceLevel)
	defer log.SetOutput(os.Stderr) // return back to default
	hook := new(test.Hook)
	log.AddHook(hook)

	javaProcessSample := types.ProcessSample{ProcessDisplayName: "java", CmdLine: "/bin/java"}

	rule := config.IncludeMetricsMap{"process.name": {"java"}}
	ec := sampler.NewMatcherChain(config.MetricsMap(rule))

	assert.Len(t, ec.Matchers, len(rule))
	assert.EqualValues(t, true, ec.Evaluate(javaProcessSample))

	require.NotEmpty(t, hook.AllEntries())
	entry := hook.LastEntry()
	assert.Equal(t, "'java' matches expression [ProcessDisplayName processDisplayName] >> 'java': true", entry.Message)
	assert.Equal(t, logrus.TraceLevel, entry.Level)
}

type enabledFFRetriever struct{}

func (e *enabledFFRetriever) GetFeatureFlag(name string) (enabled bool, exists bool) {
	return true, true
}

type disabledFFRetriever struct{}

func (e *disabledFFRetriever) GetFeatureFlag(name string) (enabled bool, exists bool) {
	return false, true
}

func TestNewSampleMatchFn(t *testing.T) {
	trueVar := true
	falseVar := false
	emptyMatchers := config.IncludeMetricsMap{}

	type args struct {
		enableProcessMetrics   *bool
		includeMetricsMatchers config.IncludeMetricsMap
		ffRetriever            feature_flags.Retriever
		sample                 interface{}
	}
	tests := []struct {
		name    string
		args    args
		include bool
	}{
		{
			name: "non process samples are always included",
			args: args{
				enableProcessMetrics:   &falseVar,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.NetworkSample,
			},
			include: true,
		},
		{
			name: "when enableProcessMetrics is FALSE process samples are always excluded",
			args: args{
				enableProcessMetrics:   &falseVar,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.ProcessSample,
			},
			include: false,
		},
		{
			name: "when enableProcessMetrics is FALSE flat process samples are always excluded",
			args: args{
				enableProcessMetrics:   &falseVar,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.FlatProcessSample,
			},
			include: false,
		},
		{
			name: "when enableProcessMetrics process samples are included",
			args: args{
				enableProcessMetrics:   &trueVar,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.ProcessSample,
			},
			include: true,
		},
		{
			name: "when enableProcessMetrics flat process samples are included",
			args: args{
				enableProcessMetrics:   &trueVar,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.FlatProcessSample,
			},
			include: true,
		},
		{
			name: "when enableProcessMetrics is not set and neither FF is, process samples are not included",
			args: args{
				enableProcessMetrics:   nil,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.ProcessSample,
			},
			include: false,
		},
		{
			name: "when enableProcessMetrics is not set and neither FF is, flat process samples are not included",
			args: args{
				enableProcessMetrics:   nil,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.FlatProcessSample,
			},
			include: false,
		},
		{
			name: "when enableProcessMetrics is not set and FF returns enabled, process samples are included",
			args: args{
				enableProcessMetrics:   nil,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            &enabledFFRetriever{},
				sample:                 &fixture.ProcessSample,
			},
			include: true,
		},
		{
			name: "when enableProcessMetrics is not set and FF returns enabled, flat process samples are included",
			args: args{
				enableProcessMetrics:   nil,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            &enabledFFRetriever{},
				sample:                 &fixture.FlatProcessSample,
			},
			include: true,
		},
		{
			name: "when enableProcessMetrics is not set and FF returns disabled, process samples are not included",
			args: args{
				enableProcessMetrics:   nil,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            &disabledFFRetriever{},
				sample:                 &fixture.ProcessSample,
			},
			include: false,
		},
		{
			name: "when enableProcessMetrics is not set and FF returns disabled, flat process samples are not included",
			args: args{
				enableProcessMetrics:   nil,
				includeMetricsMatchers: emptyMatchers,
				ffRetriever:            &disabledFFRetriever{},
				sample:                 &fixture.FlatProcessSample,
			},
			include: false,
		},
		{
			name: "process samples matching rules are included",
			args: args{
				enableProcessMetrics:   &trueVar,
				includeMetricsMatchers: config.IncludeMetricsMap{"process.name": []string{"regex \"foo.*\""}},
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.ProcessSample,
			},
			include: true,
		},
		{
			name: "flat process samples matching rules are included",
			args: args{
				enableProcessMetrics:   &trueVar,
				includeMetricsMatchers: config.IncludeMetricsMap{"process.name": []string{"regex \"foo.*\""}},
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.FlatProcessSample,
			},
			include: true,
		},
		{
			name: "process samples not matching rules are not included",
			args: args{
				enableProcessMetrics:   &trueVar,
				includeMetricsMatchers: config.IncludeMetricsMap{"process.name": []string{"regex \"bar*\""}},
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.ProcessSample,
			},
			include: false,
		},
		{
			name: "flat process samples not matching rules are not included",
			args: args{
				enableProcessMetrics:   &trueVar,
				includeMetricsMatchers: config.IncludeMetricsMap{"process.name": []string{"regex \"bar*\""}},
				ffRetriever:            testFF.EmptyFFRetriever,
				sample:                 &fixture.FlatProcessSample,
			},
			include: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchFn := sampler.NewSampleMatchFn(tt.args.enableProcessMetrics, config.MetricsMap(tt.args.includeMetricsMatchers), tt.args.ffRetriever)
			assert.Equal(t, tt.include, matchFn(tt.args.sample))
		})
	}
}

//nolint:exhaustruct
func Test_ProcessSamplingExcludes(t *testing.T) {
	t.Parallel()

	someProcessSample := &types.ProcessSample{
		ProcessDisplayName: "some-process",
	}
	// someNetworkSample := network.NetworkSample{InterfaceName: "eth0"}
	// someSystemSample := metrics.SystemSample{
	// 	CPUSample: &metrics.CPUSample{
	// 		CPUPercent: 50,
	// 	},
	// }
	// someStorageSample := storage.BaseSample{
	// 	Device: "/dev/sda1",
	// }

	boolAsPointer := func(val bool) *bool {
		return &val
	}

	type testCase struct {
		name          string
		c             *config.Config
		ff            feature_flags.Retriever
		expectInclude bool
	}
	testCases := []testCase{
		{
			name:          "Include not matching must not include",
			c:             &config.Config{EnableProcessMetrics: boolAsPointer(true), IncludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: false,
		},
		{
			name:          "Include matching should not exclude",
			c:             &config.Config{EnableProcessMetrics: boolAsPointer(true), IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: true,
		},
		{
			name:          "Exclude matching should exclude with process metrics enabled",
			c:             &config.Config{EnableProcessMetrics: boolAsPointer(true), ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: false,
		},
		{
			name:          "Exclude matching should exclude",
			c:             &config.Config{ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: false,
		},
		{
			name:          "Exclude not matching should not exclude with process metrics enabled",
			c:             &config.Config{EnableProcessMetrics: boolAsPointer(true), ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: true,
		},
		{
			name:          "Exclude not matching should not exclude",
			c:             &config.Config{ExcludeMetricsMatchers: map[string][]string{"process.name": {"does-not-match"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: false,
		},
		{
			name:          "Include matching should include even if exclude is configured with process metrics enabled",
			c:             &config.Config{EnableProcessMetrics: boolAsPointer(true), IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: true,
		},
		{
			name:          "Include matching should be include even when enable_process_metrics is not defined (nil)",
			c:             &config.Config{IncludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, ExcludeMetricsMatchers: map[string][]string{"process.name": {"some-process"}}, DisableCloudMetadata: true},
			ff:            testFF.NewFFRetrieverReturning(false, false),
			expectInclude: true,
		},
	}

	for _, tc := range testCases {
		testCase := tc
		a, _ := agent.NewAgent(testCase.c, "test", "userAgent", testCase.ff)

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expectInclude, a.Context.IncludeEvent(someProcessSample))
			// In all cases, events that are not ProcessSamples should always be included!
			// assert.True(t, a.Context.IncludeEvent(someSystemSample))
			// assert.True(t, a.Context.IncludeEvent(someNetworkSample))
			// assert.True(t, a.Context.IncludeEvent(someStorageSample))
		})
	}
}

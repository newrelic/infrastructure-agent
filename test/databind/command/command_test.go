// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build darwin dragonfly freebsd linux netbsd openbsd solaris
// +build slow

package command_test

import (
	"fmt"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exeTemplate struct {
	Ip      string
	Port    string
	Version string
}

func TestExecutableFetch(t *testing.T) {
	t.Parallel()
	vals := `[{\"variables\":{\"ip\":\"1.2.3.5\",\"port\":101,\"version\":3,\"unique_identifier\":\"123e4567e89b12d3a456426655440000\"},\"metricAnnotations\":{\"role\":\"test\",\"test_name\":\"TestRunFunc\",\"test_exe_environment_var\":\"${test_exe_environment_var}\",\"a_map\":{\"something\":\"important\",\"number\":1}},\"entityRewrites\":[{\"action\":\"replace\",\"match\":\"\${ip}\",\"replaceField\":\"container:\${unique_identifier}\"}]},{\"variables\":{\"ip\":\"1.2.3.4\",\"port\":1337,\"version\":2.1},\"metricAnnotations\":{\"test\":true,\"test_name\":\"TestRunFunc\"},\"entityRewrites\":[]}]`

	tests := []struct {
		name     string
		template string
	}{
		{
			name: "array",
			template: `
discovery:
  command:
    exec: [ 'sh', '-c', 'echo "%s"' ]
    env:
      test_exe_environment_var: executable_test
    timeout: 1m30s
    match:
      version: 3
`,
		},
		{
			name: "spaces",
			template: `
discovery:
  command:
    exec: sh -c 'echo "%s"'
    env:
      test_exe_environment_var: executable_test
    timeout: 1m30s
    match:
      version: 3
`,
		},
		{
			name: "yamlArray",
			template: `
discovery:
  command:
    exec:
    - sh
    - -c
    - echo "%s"
    env:
      test_exe_environment_var: executable_test
    timeout: 1m30s
    match:
      version: 3
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(tt.template, vals)
			t.Log(input)

			// WHEN the data is fetched
			cfg, err := databind.LoadYAML([]byte(input))
			require.NoError(t, err)
			ctx, err := databind.Fetch(cfg)
			require.NoError(t, err)
			template := exeTemplate{
				Ip:      "${discovery.ip}",
				Port:    "${discovery.port}",
				Version: "${discovery.version}",
			}
			// THEN one application is found
			matches, err := databind.Replace(&ctx, template)
			require.NoError(t, err)
			t.Log(matches)
			assert.Equal(t, "1.2.3.5", matches[0].Variables.(exeTemplate).Ip)
			assert.Equal(t, "3", matches[0].Variables.(exeTemplate).Version)
			assert.Equal(t, "101", matches[0].Variables.(exeTemplate).Port)
			assert.Equal(t, "101", matches[0].Variables.(exeTemplate).Port)
			assert.Equal(t, "test", matches[0].MetricAnnotations["role"])
			assert.Equal(t, "TestRunFunc", matches[0].MetricAnnotations["test_name"])
			assert.Equal(t, "executable_test", matches[0].MetricAnnotations["test_exe_environment_var"])
			assert.Equal(t, "important", matches[0].MetricAnnotations["a_map.something"])
			assert.Equal(t, "1", matches[0].MetricAnnotations["a_map.number"])
			require.Len(t, matches[0].EntityRewrites, 1)
			assert.Equal(t, "replace", matches[0].EntityRewrites[0].Action)
			assert.Equal(t, "container:123e4567e89b12d3a456426655440000", matches[0].EntityRewrites[0].ReplaceField)
			assert.Equal(t, "1.2.3.5", matches[0].EntityRewrites[0].Match)
		})
	}
}

func TestExecutableFetch_k8sExample(t *testing.T) {
	integration := `---
discovery:
  command:
    exec: [ 'sh', '-c', 'cat ./k8s-discovery-example.json' ]
    env:
      test_exe_environment_var: executable_test
    timeout: 1m30s
    match:
      name: redis-master
`
	// WHEN the data is fetched
	cfg, err := databind.LoadYAML([]byte(integration))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	t.Log(ctx)

	template := exeTemplate{
		Ip:      "${discovery.ip}",
		Version: "${discovery.podName}",
	}
	// THEN one application is found
	matches, err := databind.Replace(&ctx, template)
	require.NoError(t, err)
	t.Log(matches)
	assert.Equal(t, "172.17.0.7", matches[0].Variables.(exeTemplate).Ip)
	assert.Equal(t, "app-695c788f64-xpdnc", matches[0].Variables.(exeTemplate).Version)

	require.Len(t, matches[0].EntityRewrites, 1)
	assert.Equal(t, "replace", matches[0].EntityRewrites[0].Action)
	assert.Equal(t, "172.17.0.7", matches[0].EntityRewrites[0].Match)
	assert.Equal(t, "k8s:ohai-test:default:pod:app-695c788f64-xpdnc:redis-master", matches[0].EntityRewrites[0].ReplaceField)
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build fargate_inside

package fargate_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
)

// The above tests will be executed inside a container. They will be invoked from the test in fargate_test.go

type container struct {
	IP          string
	Port        string
	PrivateIP   string
	PrivatePort string
	Author      string
	Image       string
	Name        string
}

var template = container{
	IP:          "${discovery.ip}",
	Port:        "${discovery.port}",
	PrivateIP:   "${discovery.private.ip}",
	PrivatePort: "${discovery.private.port}",
	Author:      "${discovery.label.author}",
	Image:       "${discovery.image}",
	Name:        "${discovery.name}",
}

func TestFargateFetch(t *testing.T) {
	// GIVEN the public ip and port discovery of a fargate container
	input := `
discovery:
  fargate:
    match:
      name: fargate_webserver_public_1
`
	// WHEN the matches is fetched
	cfg, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	// THEN one container is found
	matches, err := databind.Replace(&ctx, template)
	require.NoError(t, err)

	require.Len(t, matches, 1)

	// AND the returned maps hold all the information, including MetricAnnotations
	ipExpr := "^(\\d\\d?\\d?\\.){3}\\d\\d?\\d?$"
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).IP)
	assert.Equal(t, "28888", matches[0].Variables.(container).Port) // values specified in fargate-compose.yml
	assert.Equal(t, "fargate_webserver_public_1", matches[0].Variables.(container).Name)
	assert.Equal(t, "httpd:2.4-alpine", matches[0].Variables.(container).Image)
	assert.Equal(t, "superman", matches[0].Variables.(container).Author)

	mas := matches[0].MetricAnnotations
	t.Log(mas)
	assert.True(t, len(mas) >= 5)
	assert.Equal(t, "httpd:2.4-alpine", mas["image"])
	assert.NotEmpty(t, mas["imageId"])
	assert.Equal(t, "fargate_webserver_public_1", mas["containerName"])
	assert.Equal(t, "fargate_webserver_public_1", mas["dockerContainerName"])
	assert.NotEmpty(t, mas["containerId"])
	assert.Equal(t, "superman", mas["label.author"])

	require.Len(t, matches[0].EntityRewrites, 1)
	assert.Equal(t, "169.254.170.4", matches[0].EntityRewrites[0].Match)
	assert.Equal(t, "replace", matches[0].EntityRewrites[0].Action)
	assert.True(t, strings.HasPrefix(matches[0].EntityRewrites[0].ReplaceField, "container:"), "Got replace field %s", matches[0].EntityRewrites[0].ReplaceField)
}

func TestFargateMultiMatch(t *testing.T) {
	// GIVEN the discovery of a fargate container that matches several containers
	input := `
discovery:
  fargate:
    match:
      name: /webserver/
`
	// WHEN the matches is fetched
	cfg, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	var template = container{
		IP:          "${discovery.ip}",
		PrivateIP:   "${discovery.private.ip}",
		PrivatePort: "${discovery.private.port}",
		Author:      "${discovery.label.author}",
		Image:       "${discovery.image}",
		Name:        "${discovery.name}",
	}

	// THEN two containers are found for the same criteria
	matches, err := databind.Replace(&ctx, template)
	require.NoError(t, err)
	require.Len(t, matches, 2)

	// AND the returned maps hold all the information
	ipExpr := "^(\\d\\d?\\d?\\.){3}\\d\\d?\\d?$"
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).IP)
	assert.Regexp(t, ipExpr, matches[1].Variables.(container).IP)
	assert.Equal(t, "80", matches[0].Variables.(container).PrivatePort) // values specified in fargate-compose.yml
	assert.Equal(t, "80", matches[1].Variables.(container).PrivatePort)
	assert.NotEqual(t, matches[0].Variables.(container).IP, matches[1].Variables.(container).IP)

	lblIndex := 0
	nonLblIndex := 1
	if matches[1].Variables.(container).Name == "fargate_webserver_private_1" {
		lblIndex, nonLblIndex = nonLblIndex, lblIndex
	}

	assert.Equal(t, "fargate_webserver_public_1", matches[nonLblIndex].Variables.(container).Name)
	assert.Equal(t, "httpd:2.4-alpine", matches[nonLblIndex].Variables.(container).Image)
	assert.Equal(t, "superman", matches[nonLblIndex].Variables.(container).Author)

	assert.Equal(t, "fargate_webserver_private_1", matches[lblIndex].Variables.(container).Name)
	assert.Equal(t, "nginx:1.17-alpine", matches[lblIndex].Variables.(container).Image)
	assert.Equal(t, "mmacias", matches[lblIndex].Variables.(container).Author)
}

func TestFargate_Match_Multicriteria(t *testing.T) {
	// GIVEN container matching criteria based on two entries
	input := `
discovery:
  fargate:
    match:
      name: /webserver/
      label.author: /.*i.s/
`
	// WHEN the data is fetched
	cfg, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	noPortTempl := template
	noPortTempl.Port = ""

	// THEN only the container that matches all the criteria is found
	matches, err := databind.Replace(&ctx, noPortTempl)
	require.NoError(t, err)
	assert.Len(t, matches, 1)

	// AND the returned maps hold all the information
	ipExpr := "^(\\d\\d?\\d?\\.){3}\\d\\d?\\d?$"
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).IP)
	assert.Equal(t, "80", matches[0].Variables.(container).PrivatePort) // values specified in docker-compose.yml
	assert.Equal(t, "mmacias", matches[0].Variables.(container).Author)
	assert.Equal(t, "fargate_webserver_private_1", matches[0].Variables.(container).Name)
	assert.Equal(t, "nginx:1.17-alpine", matches[0].Variables.(container).Image)
}

func TestFargate_Match_ByIPAndPort(t *testing.T) {
	// GIVEN container matching criteria based the open public port
	input := `
discovery:
  fargate:
    match:
      private.ip: 169.254.170.4
      private.port: 80
`
	// WHEN the data is fetched
	cfg, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	// THEN only the container that matches all the criteria is found
	matches, err := databind.Replace(&ctx, template)
	require.NoError(t, err)
	assert.Len(t, matches, 1)

	// AND the returned maps hold all the information
	assert.Equal(t, "169.254.170.4", matches[0].Variables.(container).PrivateIP)
	assert.Equal(t, "169.254.170.4", matches[0].Variables.(container).IP)
	assert.Equal(t, "80", matches[0].Variables.(container).PrivatePort) // values specified in docker-compose.yml
	assert.Equal(t, "28888", matches[0].Variables.(container).Port)     // values specified in docker-compose.yml

}

func TestFargate_PortIndexing(t *testing.T) {
	input := `
discovery:
  fargate:
    match:
      image: /^httpd/
`
	template := map[string]string{
		"port":                "${discovery.port}",
		"ports.0":             "${discovery.ports.0}",
		"ports.1":             "${discovery.ports.1}",
		"ports.tcp":           "${discovery.ports.tcp}",
		"ports.tcp.0":         "${discovery.ports.tcp.0}",
		"ports.tcp.1":         "${discovery.ports.tcp.1}",
		"private.port":        "${discovery.private.port}",
		"private.ports.0":     "${discovery.private.ports.0}",
		"private.ports.1":     "${discovery.private.ports.1}",
		"private.ports.tcp":   "${discovery.private.ports.tcp}",
		"private.ports.tcp.0": "${discovery.private.ports.tcp.0}",
		"private.ports.tcp.1": "${discovery.private.ports.tcp.1}",
	}
	expected := map[string]string{
		"port":                "28888",
		"ports.0":             "28888",
		"ports.1":             "8443",
		"ports.tcp":           "28888",
		"ports.tcp.0":         "28888",
		"ports.tcp.1":         "8443",
		"private.port":        "80",
		"private.ports.0":     "80",
		"private.ports.1":     "443",
		"private.ports.tcp":   "80",
		"private.ports.tcp.0": "80",
		"private.ports.tcp.1": "443",
	}
	cfg, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	// THEN only the container that matches all the criteria is found
	matches, err := databind.Replace(&ctx, template)
	require.NoError(t, err)
	assert.Len(t, matches, 1)
	for name, value := range matches[0].Variables.(map[string]string) {
		assert.Equalf(t, expected[name], value, "not matching value for port named %q", name)
	}
}

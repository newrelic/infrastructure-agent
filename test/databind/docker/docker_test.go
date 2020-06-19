// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build slow

package docker_test

import (
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	test "github.com/newrelic/infrastructure-agent/test/databind"
)

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

func TestMain(m *testing.M) {
	if err := test.ComposeUp("./docker-compose.yml"); err != nil {
		log.Println("error on compose-up: ", err.Error())
		os.Exit(-1)
	}
	exitValChn := make(chan int, 1)
	func() {
		defer test.ComposeDown("./docker-compose.yml")
		exitValChn <- m.Run()
	}()

	exitVal := <-exitValChn
	os.Exit(exitVal)
}

func TestDockerFetch(t *testing.T) {
	// GIVEN the public ip and port discovery of a docker container
	input := `
discovery:
  docker:
    match:
      name: docker_webserver_public_1
`
	// WHEN the data is fetched
	cfg, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	// THEN one container is found
	matches, err := databind.Replace(&ctx, template)
	require.NoError(t, err)

	assert.Len(t, matches, 1)

	// AND the returned maps hold all the information, including MetricAnnotations
	ipExpr := "^(\\d\\d?\\d?\\.){3}\\d\\d?\\d?$"
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).IP)
	assert.Equal(t, "18888", matches[0].Variables.(container).Port) // values specified in docker-compose.yml
	assert.Equal(t, "docker_webserver_public_1", matches[0].Variables.(container).Name)
	assert.Equal(t, "httpd:2.4-alpine", matches[0].Variables.(container).Image)
	assert.Equal(t, "superman", matches[0].Variables.(container).Author)

	mas := matches[0].MetricAnnotations
	assert.True(t, len(mas) >= 5)
	assert.Equal(t, "httpd:2.4-alpine", mas["image"])
	assert.NotEmpty(t, mas["imageId"])
	assert.Equal(t, "docker_webserver_public_1", mas["containerName"])
	assert.Equal(t, "httpd-foreground", mas["command"])
	assert.NotEmpty(t, mas["containerId"])
	assert.Equal(t, "superman", mas["label.author"])

	require.Len(t, matches[0].EntityRewrites, 1)
	// should be 0.0.0.0 as exposed on local host
	assert.Equal(t, "0.0.0.0", matches[0].EntityRewrites[0].Match)
	assert.Equal(t, "replace", matches[0].EntityRewrites[0].Action)
	assert.True(t, strings.HasPrefix(matches[0].EntityRewrites[0].ReplaceField, "container:"), "Got replace field %s", matches[0].EntityRewrites[0].ReplaceField)
}

func TestMultiMatch(t *testing.T) {
	// GIVEN the discovery of a docker container that matches several containers
	input := `
discovery:
  docker:
    match:
      name: /webserver/
`
	// WHEN the data is fetched
	cfg, err := databind.LoadYAML([]byte(input))
	require.NoError(t, err)
	ctx, err := databind.Fetch(cfg)
	require.NoError(t, err)

	// THEN two containers are found for the same criteria
	matches, err := databind.Replace(&ctx, template)
	require.NoError(t, err)

	assert.Len(t, matches, 2)

	// AND the returned maps hold all the information
	ipExpr := "^(\\d\\d?\\d?\\.){3}\\d\\d?\\d?$"
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).PrivateIP)
	assert.Regexp(t, ipExpr, matches[1].Variables.(container).PrivateIP)
	assert.Equal(t, "80", matches[0].Variables.(container).PrivatePort) // values specified in docker-compose.yml
	assert.Equal(t, "80", matches[1].Variables.(container).PrivatePort)
	assert.NotEqual(t, matches[0].Variables.(container).PrivateIP, matches[1].Variables.(container).PrivateIP)

	lblIndex := 0
	nonLblIndex := 1
	if matches[1].Variables.(container).Name == "docker_webserver_private_1" {
		lblIndex, nonLblIndex = nonLblIndex, lblIndex
	}
	assert.Equal(t, "docker_webserver_private_1", matches[lblIndex].Variables.(container).Name)
	assert.Equal(t, "nginx:1.17-alpine", matches[lblIndex].Variables.(container).Image)
	assert.Equal(t, "mmacias", matches[lblIndex].Variables.(container).Author)
	assert.Equal(t, "docker_webserver_public_1", matches[nonLblIndex].Variables.(container).Name)
	assert.Equal(t, "18888", matches[nonLblIndex].Variables.(container).Port)
	assert.Equal(t, "httpd:2.4-alpine", matches[nonLblIndex].Variables.(container).Image)
	assert.Equal(t, "superman", matches[nonLblIndex].Variables.(container).Author)
}

func TestMatch_Multicriteria(t *testing.T) {
	// GIVEN container matching criteria based on two entries
	input := `
discovery:
  docker:
    match:
      name: /webserver/
      label.author: /.*i.s/
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
	ipExpr := "^(\\d\\d?\\d?\\.){3}\\d\\d?\\d?$"
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).PrivateIP)
	assert.Equal(t, "80", matches[0].Variables.(container).PrivatePort) // values specified in docker-compose.yml
	assert.Equal(t, "mmacias", matches[0].Variables.(container).Author)
}

func TestMatch_ByPort(t *testing.T) {
	// GIVEN container matching criteria based the open public port
	input := `
discovery:
  docker:
    match:
      port: 18888
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
	ipExpr := "^(\\d\\d?\\d?\\.){3}\\d\\d?\\d?$"
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).PrivateIP)
	assert.Regexp(t, ipExpr, matches[0].Variables.(container).IP)
	assert.Equal(t, "80", matches[0].Variables.(container).PrivatePort) // values specified in docker-compose.yml
	assert.Equal(t, "18888", matches[0].Variables.(container).Port)     // values specified in docker-compose.yml
}

func TestDocker_PortIndexing(t *testing.T) {
	input := `
discovery:
  docker:
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
		"port":                "18888",
		"ports.0":             "18888",
		"ports.1":             "8443",
		"ports.tcp":           "18888",
		"ports.tcp.0":         "18888",
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

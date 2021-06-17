// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config_loader

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/databind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestFile(data []byte) (*os.File, error) {
	tmp, err := ioutil.TempFile("", "loadconfig")
	if err != nil {
		return nil, err
	}
	_, err = tmp.Write(data)
	if err != nil {
		return nil, err
	}
	tmp.Close()
	return tmp, nil
}

func TestLoadYamlConfig(t *testing.T) {
	yamlData := []byte(`param: hello`)

	tmp, err := createTestFile(yamlData)
	require.NoError(t, err)
	defer os.Remove(tmp.Name())

	var cfg struct {
		Param string `yaml:"param"`
	}

	meta, err := LoadYamlConfig(&cfg, tmp.Name())
	require.NoError(t, err)
	assert.Equal(t, "hello", cfg.Param)
	require.NotNil(t, meta)
	assert.True(t, meta.Contains("param"))
	assert.False(t, meta.Contains("otherParam"))
}

func TestMissingLoadYamlConfig(t *testing.T) {
	cfg := &struct {
		Param string `yaml:"param"`
	}{}

	meta, err := LoadYamlConfig(&cfg, "idontexist.yml")
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Param)
	require.NotNil(t, meta)
	assert.False(t, meta.Contains("param"))
	assert.False(t, meta.Contains("otherParam"))
}

func TestLoadYamlConfig_withDatabindVariables(t *testing.T) {
	yamlData := []byte(`
variables:
  creds:
    vault:
      http:
        url: http://my.vault.host/v1/newengine/data/secret
        headers:
          X-Vault-Token: my-vault-token
foo: bar
baz: ${creds.user}
`)

	tmp, err := createTestFile(yamlData)
	require.NoError(t, err)
	defer os.Remove(tmp.Name())

	var cfg struct {
		Foo      string                   `yaml:"foo"`
		Baz      string                   `yaml:"baz"`
		Databind databind.YAMLAgentConfig `yaml:",inline"`
	}

	meta, err := LoadYamlConfig(&cfg, tmp.Name())

	require.NoError(t, err)

	assert.Equal(t, YAMLMetadata{"variables": true, "baz": true, "foo": true}, *meta)

	assert.Equal(t, "bar", cfg.Foo)
	assert.Equal(t, "${creds.user}", cfg.Baz)
	require.Contains(t, cfg.Databind.Variables, "creds")
	assert.Equal(t, cfg.Databind.Variables["creds"].Vault.HTTP.URL, "http://my.vault.host/v1/newengine/data/secret")
}

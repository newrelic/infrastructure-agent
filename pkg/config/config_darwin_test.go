// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build darwin
// +build darwin

package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	// Test that missing fields are replaced by its default
	configStr := `
license_key: abc123
log:
  level: debug
custom_attributes:
  test: test
`
	f, err := ioutil.TempFile("", "default_config_test")
	assert.NoError(t, err)

	n, err := f.WriteString(configStr)
	assert.NoError(t, err)
	assert.EqualValues(t, n, len(configStr))

	err = f.Close()
	assert.NoError(t, err)

	cfg, err := LoadConfig(f.Name())
	assert.NoError(t, err)
	assert.Equal(t, "debug", cfg.Log.Level)

	assert.Equal(t, 0, *cfg.Log.Rotate.MaxSizeMb)
	assert.Equal(t, 0, cfg.Log.Rotate.MaxFiles)
	assert.Equal(t, false, cfg.Log.Rotate.CompressionEnabled)
	assert.Equal(t, "", cfg.Log.Rotate.FilePattern)

	assert.Equal(t, os.TempDir(), cfg.AgentTempDir)
}

func TestRotateConfig(t *testing.T) {
	// Test that missing fields are replaced by its default
	configStr := `
license_key: abc123
log:
  level: debug
  include_filters:  
    component:
      - ProcessSample
      - StorageSample
  rotate:
    max_size_mb: 10
    max_files: 20
    compression_enabled: true
    file_pattern: pattern.log
    
`
	f, err := ioutil.TempFile("", "default_config_test")
	assert.NoError(t, err)

	n, err := f.WriteString(configStr)
	assert.NoError(t, err)
	assert.EqualValues(t, n, len(configStr))

	err = f.Close()
	assert.NoError(t, err)

	cfg, err := LoadConfig(f.Name())
	assert.NoError(t, err)
	assert.Equal(t, "debug", cfg.Log.Level)

	assert.Equal(t, 10, *cfg.Log.Rotate.MaxSizeMb)
	assert.Equal(t, 20, cfg.Log.Rotate.MaxFiles)
	assert.Equal(t, true, cfg.Log.Rotate.CompressionEnabled)
	assert.Equal(t, "pattern.log", cfg.Log.Rotate.FilePattern)
}

func Test_ParseLogConfigRule_EnvVar(t *testing.T) {
	os.Setenv("NRIA_LOG_FILE", "agent.log")
	defer os.Unsetenv("NRIA_LOG_FILE")
	os.Setenv("NRIA_LOG_LEVEL", "smart")
	defer os.Unsetenv("NRIA_LOG_LEVEL")
	os.Setenv("NRIA_LOG_FORMAT", "json")
	defer os.Unsetenv("NRIA_LOG_FORMAT")
	os.Setenv("NRIA_LOG_STDOUT", "false")
	defer os.Unsetenv("NRIA_LOG_STDOUT")
	os.Setenv("NRIA_LOG_SMART_LEVEL_ENTRY_LIMIT", "50")
	defer os.Unsetenv("NRIA_LOG_SMART_LEVEL_ENTRY_LIMIT")
	os.Setenv("NRIA_LOG_INCLUDE_FILTERS", "component:\n - ProcessSample\n - StorageSample\n")
	defer os.Unsetenv("NRIA_LOG_INCLUDE_FILTERS")
	intPtr := func(a int) *int {
		return &a
	}
	configStr := "license_key: abc123"
	f, err := ioutil.TempFile("", "yaml_config_test")
	assert.NoError(t, err)
	f.WriteString(configStr)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	assert.NoError(t, err)
	expectedStdout := false
	expectedSmartLevelEntryLimit := 50
	expected := LogConfig{
		File:                 "agent.log",
		Level:                LogLevelSmart,
		Format:               "json",
		ToStdout:             &expectedStdout,
		Forward:              nil,
		SmartLevelEntryLimit: &expectedSmartLevelEntryLimit,
		IncludeFilters:       map[string][]interface{}{"component": {"ProcessSample", "StorageSample"}},
		ExcludeFilters: map[string][]interface{}{
			TracesFieldName: {SupervisorTrace, FeatureTrace, ProcessTrace}, IntegrationsErrorsField: {IntegrationsErrorsValue},
		},
		Rotate: LogRotateConfig{
			MaxSizeMb:          intPtr(0),
			MaxFiles:           0,
			CompressionEnabled: false,
			FilePattern:        "",
		},
	}
	assert.EqualValues(t, expected, cfg.Log)
}

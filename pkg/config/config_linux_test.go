// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	. "gopkg.in/check.v1"
)

func (s *ConfigSuite) TestParseConfigOverride(c *C) {
	config := `
compaction_threshold: 54
daemontools_refresh_sec: 32
verbose: 1
ignored_inventory:
   - files/config/stuff.bar
   - files/config/stuff.foo
license_key: abc123
custom_attributes:
   my_group:  test group
   agent_role:  test role
debug: false
overide_host_root: /dockerland
is_containerized: false
`
	f, err := ioutil.TempFile("", "opsmatic_config_test_2")
	c.Assert(err, IsNil)
	_, _ = f.WriteString(config)
	_ = f.Close()

	cfg, err := LoadConfig(f.Name())
	c.Assert(err, IsNil)

	c.Assert(os.Getenv("HOST_ETC"), Equals, "/dockerland/etc")
	c.Assert(cfg.IsContainerized, Equals, false)
	c.Assert(cfg.IsForwardOnly, Equals, false)
	c.Assert(cfg.IsSecureForwardOnly, Equals, false)
	c.Assert(cfg.IsIntegrationsOnly, Equals, false)

	_ = os.Setenv("NRIA_LICENSE_KEY", "abcd1234")
	_ = os.Setenv("NRIA_COMPACTION_THRESHOLD", "55")
	_ = os.Setenv("NRIA_DAEMONTOOLS_INTERVAL_SEC", "33")
	_ = os.Setenv("NRIA_VERBOSE", "0")
	_ = os.Setenv("NRIA_DEBUG", "false")
	_ = os.Setenv("NRIA_IGNORED_INVENTORY", "files/config/things.bar,files/config/things.foo")
	_ = os.Setenv("NRIA_CUSTOM_ATTRIBUTES",
		`{"my_groups":"testing group", "agent_roles":"testing role"}`)
	_ = os.Setenv("NRIA_OVERRIDE_HOST_ETC", "/opt/etc")
	_ = os.Setenv("NRIA_OVERRIDE_HOST_PROC", "/docker_proc")
	_ = os.Setenv("NRIA_OVERRIDE_HOST_ROOT", "/dockerworld")
	_ = os.Setenv("NRIA_OVERRIDE_HOST_SYS", "/docker_sys")
	_ = os.Setenv("NRIA_IS_CONTAINERIZED", "true")
	_ = os.Setenv("NRIA_IS_FORWARD_ONLY", "true")
	_ = os.Setenv("NRIA_IS_SECURE_FORWARD_ONLY", "true")
	_ = os.Setenv("NRIA_IS_INTEGRATIONS_ONLY", "true")

	defer func() {
		_ = os.Unsetenv("NRIA_LICENSE_KEY")
		_ = os.Unsetenv("NRIA_COMPACTION_THRESHOLD")
		_ = os.Unsetenv("NRIA_DAEMONTOOLS_REFRESH_SEC")
		_ = os.Unsetenv("NRIA_VERBOSE")
		_ = os.Unsetenv("NRIA_DEBUG")
		_ = os.Unsetenv("NRIA_IGNORED_INVENTORY")
		_ = os.Unsetenv("NRIA_CUSTOM_ATTRIBUTES")
		_ = os.Unsetenv("NRIA_OVERRIDE_HOST_ETC")
		_ = os.Unsetenv("NRIA_OVERRIDE_HOST_PROC")
		_ = os.Unsetenv("NRIA_OVERRIDE_HOST_ROOT")
		_ = os.Unsetenv("NRIA_OVERRIDE_HOST_SYS")
		_ = os.Unsetenv("NRIA_IS_CONTAINERIZED")
		_ = os.Unsetenv("NRIA_IS_FORWARD_ONLY")
		_ = os.Unsetenv("NRIA_IS_SECURE_FORWARD_ONLY")
		_ = os.Unsetenv("NRIA_IS_INTEGRATIONS_ONLY")

		_ = os.Unsetenv("HOST_SYS")
		_ = os.Unsetenv("HOST_ETC")
		_ = os.Unsetenv("HOST_PROC")
	}()

	configOverride(cfg)
	c.Log(cfg.CustomAttributes)
	c.Assert(cfg.License, Equals, "abcd1234")
	c.Assert(cfg.CompactThreshold, Equals, uint64(55))
	c.Assert(cfg.DaemontoolsRefreshSec, Equals, int64(33))
	c.Assert(cfg.Verbose, Equals, 0)
	c.Assert(cfg.IgnoredInventoryPaths, DeepEquals, []string{"files/config/things.bar", "files/config/things.foo"})
	c.Assert(cfg.CustomAttributes, DeepEquals, CustomAttributeMap{
		"my_groups":   "testing group",
		"agent_roles": "testing role",
	})
	c.Assert(cfg.OverrideHostSys, Equals, "/dockerworld/docker_sys")
	c.Assert(cfg.OverrideHostProc, Equals, "/dockerworld/docker_proc")
	c.Assert(cfg.OverrideHostEtc, Equals, "/dockerworld/opt/etc")
	c.Assert(os.Getenv("HOST_ETC"), Equals, "/dockerworld/opt/etc")
	c.Assert(os.Getenv("HOST_PROC"), Equals, "/dockerworld/docker_proc")
	c.Assert(os.Getenv("HOST_SYS"), Equals, "/dockerworld/docker_sys")
	c.Assert(cfg.IsContainerized, Equals, true)
	c.Assert(cfg.IsForwardOnly, Equals, true)
	c.Assert(cfg.IsSecureForwardOnly, Equals, true)
	c.Assert(cfg.IsIntegrationsOnly, Equals, true)
}

func TestDefaultConfig(t *testing.T) {
	// Test that missing fields are replaced by its default
	configStr := `
license_key: abc123
log:
  level: debug
    
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

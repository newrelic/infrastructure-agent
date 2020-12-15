// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "gopkg.in/check.v1"
)

type ConfigSuite struct {
}

var _ = Suite(&ConfigSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *ConfigSuite) TestParseConfig(c *C) {
	config := `
collector_url:  http://url.test
ignored_inventory:
   - files/config/stuff.bar
   - files/config/stuff.foo
license_key: abc123
custom_attributes:
   my_group:  test group
   agent_role:  test role
remove_entities_period: 1h
`
	f, err := ioutil.TempFile("", "opsmatic_config_test")
	c.Assert(err, IsNil)
	f.WriteString(config)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	c.Assert(err, IsNil)
	c.Assert(cfg.MetricURL, Equals, "http://url.test")
	c.Assert(cfg.CollectorURL, Equals, "http://url.test")
	c.Assert(cfg.License, Equals, "abc123")
	c.Assert(cfg.IgnoredInventoryPaths, HasLen, 2)
	c.Assert(
		cfg.IgnoredInventoryPathsMap,
		DeepEquals,
		map[string]struct{}{
			"files/config/stuff.bar": {},
			"files/config/stuff.foo": {},
		},
	)
	c.Assert(cfg.CustomAttributes, HasLen, 2)
	c.Assert(cfg.CustomAttributes["my_group"], Equals, "test group")
	c.Assert(cfg.CustomAttributes["agent_role"], Equals, "test role")
	c.Assert(cfg.RemoveEntitiesPeriod, Equals, "1h")
}

func (s *ConfigSuite) TestParseConfigBadLicense(c *C) {
	keyTest := []struct {
		inputKey  string
		parsedKey string
		isBad     bool
	}{
		{"bad_with_underscore", "bad_with_underscore", true},
		{"<angles are bad>", "<angles are bad>", true},
		{"the word lice as in license or licence is bad", "the word lice as in license or licence is bad", true},
		{"dont use relic", "dont use relic", true},
		{"CAP RELIC ARE BAD", "CAP RELIC ARE BAD", true},
		{"don't use single quote", "don't use single quote", true},
		{"do not use period.", "do not use period.", true},
		{"<abcdef12345667890>", "<abcdef12345667890>", true},
		{".abcdef12345667890", ".abcdef12345667890", true},
		{"'abcdef12345667890'", "abcdef12345667890", false},
		{"abcdef12345667890", "abcdef12345667890", false},
		{"ABCDEF12345667890", "ABCDEF12345667890", false},
		{"    fabcdef12345667890  ", "fabcdef12345667890", false},
		{"XYZabcdef12345667890", "XYZabcdef12345667890", false},
		{"0123456789012345678901234567890123456789", "0123456789012345678901234567890123456789", false},
		{"eu01xx6789012345678901234567890123456789", "eu01xx6789012345678901234567890123456789", false},
		{"gov01x6789012345678901234567890123456789", "gov01x6789012345678901234567890123456789", false},
	}

	config := `
collector_url:  http://foo.bar
license_key: `
	for _, test := range keyTest {
		finalConfig := fmt.Sprintf("%s%s", config, test.inputKey)
		f, err := ioutil.TempFile("", "opsmatic_config_test")
		c.Assert(err, IsNil)
		f.WriteString(finalConfig)
		f.Close()

		cfg, err := LoadConfig(f.Name())
		c.Logf("Testing %+v", test)
		if test.isBad {
			c.Assert(err, NotNil)
		} else {
			c.Assert(err, IsNil)
		}
		c.Assert(cfg.License, Equals, test.parsedKey)
	}

}

func (s *ConfigSuite) TestValidateConfigFrequencySettings(c *C) {

	var testCases = []struct {
		req      int64
		min      int64
		def      int64
		disable  bool
		expected time.Duration
	}{
		// Request is larger than min, so use request
		{10, 5, 13, false, time.Duration(10)},
		// Request is same as min, so use request
		{10, 10, 13, false, time.Duration(10)},
		// Request is smaller than min and greater than
		// FREQ_DISABLE_SAMPLING, so use def
		{5, 10, 13, false, time.Duration(13)},
		// Request is FREQ_DEFAULT_SAMPLING, so use default
		{FREQ_DEFAULT_SAMPLING, 10, 13, false, time.Duration(13)},
		// Request is less than FREQ_DISABLE_SAMPLING returns
		// FREQ_DISABLE_SAMPLING, meaning OFF
		{FREQ_DISABLE_SAMPLING - 1, 10, 13, false, time.Duration(FREQ_DISABLE_SAMPLING)},
		// Request is FREQ_DEFAULT_SAMPLING, so use default
		{FREQ_DEFAULT_SAMPLING, 10, 13, false, time.Duration(13)},
		// Disable with default returns FREQ_DISABLE_SAMPLING
		{FREQ_DEFAULT_SAMPLING, 10, 13, true, time.Duration(FREQ_DISABLE_SAMPLING)},
		// Disable with required different than FREQ_DEFAULT_SAMPLING
		// ignores disable
		{10, 5, 13, true, time.Duration(10)},
	}
	for _, tc := range testCases {
		c.Assert(
			ValidateConfigFrequencySetting(tc.req, tc.min, tc.def, tc.disable),
			Equals,
			time.Duration(tc.expected),
		)
	}
}

func (s *ConfigSuite) TestJitterFrequency(c *C) {
	var t time.Duration

	// Less than 1 second == 1 second
	t = time.Millisecond
	c.Assert(JitterFrequency(t), Equals, time.Second)

	t = time.Second - time.Hour
	c.Assert(JitterFrequency(t), Equals, time.Second)

	t = time.Duration(3) * time.Second
	j := JitterFrequency(t) / time.Second
	if j < 1 || j > 3 {
		c.Errorf("Jitter outside of range failure: 1 < %v < 3", j)
	}
}

func (s *ConfigSuite) TestYaml(c *C) {
	configStr := `
license_key: abc123
startup_connection_timeout: 33s
startup_connection_retries: 10
win_process_priority_class: "Normal"
max_procs: 3
ignore_system_proxy: true
override_hostname: superhost.newrelic.com
override_hostname_short: superhost
dns_hostname_resolution: false
ignore_reclaimable: true
proxy_validate_certificates: true
win_removable_drives: false
proxy_config_plugin: false
trunc_text_values: false
`
	f, err := ioutil.TempFile("", "yaml_config_test")
	c.Assert(err, IsNil)
	f.WriteString(configStr)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	c.Assert(err, IsNil)
	c.Assert(cfg.StartupConnectionRetries, Equals, 10)
	c.Assert(cfg.StartupConnectionTimeout, Equals, "33s")
	c.Assert(cfg.WinProcessPriorityClass, Equals, "Normal")
	c.Assert(cfg.MaxProcs, Equals, 3)
	c.Assert(cfg.IgnoreSystemProxy, Equals, true)
	c.Assert(cfg.OverrideHostname, Equals, "superhost.newrelic.com")
	c.Assert(cfg.OverrideHostnameShort, Equals, "superhost")
	c.Assert(cfg.DnsHostnameResolution, Equals, false)
	c.Assert(cfg.IgnoreReclaimable, Equals, true)
	c.Assert(cfg.ProxyValidateCerts, Equals, true)
	c.Assert(cfg.WinRemovableDrives, Equals, false)
	c.Assert(cfg.ProxyConfigPlugin, Equals, false)
	c.Assert(cfg.TruncTextValues, Equals, false)
}

func (s *ConfigSuite) TestEnv(c *C) {
	configStr := `
license_key: abc123
`
	os.Setenv("NRIA_IGNORE_RECLAIMABLE", "true")
	defer os.Unsetenv("NRIA_IGNORE_RECLAIMABLE")
	os.Setenv("NRIA_PROXY_VALIDATE_CERTIFICATES", "true")
	defer os.Unsetenv("NRIA_PROXY_VALIDATE_CERTIFICATES")
	os.Setenv("NRIA_INCLUDE_MATCHING_METRICS", "process.name:\n - regex \"kube*\" \n")
	defer os.Unsetenv("NRIA_INCLUDE_MATCHING_METRICS")

	f, err := ioutil.TempFile("", "yaml_config_test")
	c.Assert(err, IsNil)
	f.WriteString(configStr)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	c.Assert(err, IsNil)
	c.Assert(cfg.IgnoreReclaimable, Equals, true)
	c.Assert(cfg.ProxyValidateCerts, Equals, true)
	c.Assert(fmt.Sprintf("%v", cfg.IncludeMetricsMatchers), Equals, "map[process.name:[regex \"kube*\"]]")
}

func (s *ConfigSuite) TestWrongFormatDurations(c *C) {
	// Given wrong duration format
	configStr := `
license_key: abc123
startup_connection_timeout: a duck
startup_connection_retry_time: cow and pineapples
`
	f, err := ioutil.TempFile("", "wrong_yaml_config_test")
	c.Assert(err, IsNil)
	f.WriteString(configStr)
	f.Close()

	// They are reverted to default values (showing a warning into stdiout)
	cfg, err := LoadConfig(f.Name())
	c.Assert(err, IsNil)
	c.Assert(cfg.StartupConnectionTimeout, Equals, defaultStartupConnectionTimeout)
}

func (s *ConfigSuite) TestEscapedString(c *C) {
	configStr := `
license_key: abc123
custom_attributes:
   attr1: some unescaped\nstring\here
   attr2: "some escaped\nstring\\here"
   attr3: 'other unescaped\nstring\here'
   attr4: >
      Multiline string
      ignoring breaks
      and indents
   attr5: |
      Multiline string
      ignoring indents
`
	f, err := ioutil.TempFile("", "opsmatic_config_test")
	c.Assert(err, IsNil)
	f.WriteString(configStr)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	c.Assert(err, IsNil)
	c.Assert(cfg.License, Equals, "abc123")
	c.Assert(cfg.CustomAttributes, HasLen, 5)
	c.Assert(cfg.CustomAttributes["attr1"], Equals, "some unescaped\\nstring\\here")
	c.Assert(cfg.CustomAttributes["attr2"], Equals, "some escaped\nstring\\here")
	c.Assert(cfg.CustomAttributes["attr3"], Equals, "other unescaped\\nstring\\here")
	c.Assert(cfg.CustomAttributes["attr4"], Equals, "Multiline string ignoring breaks and indents\n")
	c.Assert(cfg.CustomAttributes["attr5"], Equals, "Multiline string\nignoring indents\n")
}

func (s *ConfigSuite) TestDefaultConfig(c *C) {
	// Test that missing fields are replaced by its default
	configStr := `
license_key: abc123
agent_dir: /my/overriden/agent/dir
`
	f, err := ioutil.TempFile("", "opsmatic_config_test")
	c.Assert(err, IsNil)
	f.WriteString(configStr)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	c.Assert(cfg.PidFile, Equals, defaultPidFile)
	c.Assert(cfg.MetricURL, Equals, "https://metric-api.newrelic.com")
	c.Assert(cfg.CollectorURL, Equals, "https://infra-api.newrelic.com")
	c.Assert(cfg.AgentDir, Equals, "/my/overriden/agent/dir")
	c.Assert(cfg.AppDataDir, Equals, defaultAppDataDir)
	c.Assert(cfg.LogToStdout, Equals, defaultLogToStdout)
	c.Assert(cfg.DisableWinSharedWMI, Equals, defaultDisableWinSharedWMI)
	c.Assert(cfg.EnableWinUpdatePlugin, Equals, defaultWinUpdatePlugin)
	c.Assert(cfg.PayloadCompressionLevel, Equals, defaultPayloadCompressionLevel)
	c.Assert(cfg.CompactEnabled, Equals, defaultCompactEnabled)
	c.Assert(cfg.CompactThreshold, Equals, uint64(defaultCompactThreshold))
	c.Assert(cfg.FilesConfigOn, Equals, defaultFilesConfigOn)
	c.Assert(cfg.SupervisorRpcSocket, Equals, defaultSupervisorRpcSock)
	c.Assert(cfg.DebugLogSec, Equals, defaultDebugLogSec)
	c.Assert(cfg.StripCommandLine, Equals, DefaultStripCommandLine)
	c.Assert(cfg.NetworkInterfaceFilters, DeepEquals, defaultNetworkInterfaceFilters)
	c.Assert(cfg.HTTPServerHost, Equals, defaultHTTPServerHost)
	c.Assert(cfg.HTTPServerPort, Equals, defaultHTTPServerPort)
	c.Assert(cfg.OfflineTimeToReset, Equals, DefaultOfflineTimeToReset)
	c.Assert(cfg.StartupConnectionTimeout, Equals, defaultStartupConnectionTimeout)
	c.Assert(cfg.StartupConnectionRetries, Equals, defaultStartupConnectionRetries)
	c.Assert(cfg.MaxInventorySize, Equals, defaultMaxInventorySize)
	c.Assert(cfg.DisableInventorySplit, Equals, defaultDisableInventorySplit)
	c.Assert(cfg.MaxProcs, Equals, defaultMaxProcs)
	c.Assert(cfg.IgnoreSystemProxy, Equals, false)
	c.Assert(cfg.MaxMetricBatchEntitiesCount, Equals, 300)
	c.Assert(cfg.MaxMetricBatchEntitiesQueue, Equals, 1000)

	var expectedMetricEndpoint = defaultMetricsIngestEndpoint
	if cfg.ConnectEnabled {
		expectedMetricEndpoint = defaultMetricsIngestV2Endpoint
	}
	c.Assert(cfg.MetricsIngestEndpoint, Equals, expectedMetricEndpoint)
	c.Assert(cfg.InventoryIngestEndpoint, Equals, defaultInventoryIngestEndpoint)
	c.Assert(cfg.IdentityIngestEndpoint, Equals, defaultIdentityIngestEndpoint)

	// Checking there are not empty and duplicate entries in the plugins directories
	deduped := helpers.RemoveEmptyAndDuplicateEntries(cfg.PluginInstanceDirs)
	c.Assert(cfg.PluginInstanceDirs, DeepEquals, deduped)

	c.Assert(cfg.OverrideHostname, Equals, "")
	c.Assert(cfg.OverrideHostnameShort, Equals, "")

	c.Assert(cfg.FirstReapInterval, Equals, defaultFirstReapInterval)
	c.Assert(cfg.ReapInterval, Equals, defaultReapInterval)
	c.Assert(cfg.SendInterval, Equals, defaultSendInterval)

	c.Assert(cfg.DockerApiVersion, Equals, DefaultDockerApiVersion)
	c.Assert(cfg.SelinuxEnableSemodule, Equals, defaultSelinuxEnableSemodule)
	c.Assert(cfg.DnsHostnameResolution, Equals, defaultDnsHostnameResolution)
	c.Assert(cfg.IgnoreReclaimable, Equals, defaultIgnoreReclaimable)
	c.Assert(cfg.ProxyValidateCerts, Equals, defaultProxyValidateCerts)
	c.Assert(cfg.ProxyConfigPlugin, Equals, defaultProxyConfigPlugin)
	c.Assert(cfg.TruncTextValues, Equals, defaultTruncTextValues)

	if runtime.GOOS == "windows" {
		c.Assert(cfg.WinRemovableDrives, Equals, defaultWinRemovableDrives)
	}
}

func (s *ConfigSuite) TestCalculateCollectorURL(c *C) {
	testcases := []struct {
		license   string
		expectURL string
		staging   bool
	}{
		// non-region license, staging false
		{license: "0123456789012345678901234567890123456789", expectURL: "https://infra-api.newrelic.com", staging: false},
		// non-region license, staging true
		{license: "0123456789012345678901234567890123456789", expectURL: "https://staging-infra-api.newrelic.com", staging: true},
		// four letter region
		{license: "eu01xx6789012345678901234567890123456789", expectURL: "https://infra-api.eu.newrelic.com", staging: false},
		// four letter region
		{license: "eu01xx6789012345678901234567890123456789", expectURL: "https://staging-infra-api.eu.newrelic.com", staging: true},
		//// five letter region
		{license: "gov01x6789012345678901234567890123456789", expectURL: "https://gov-infra-api.newrelic.com", staging: true},
	}

	for _, tc := range testcases {
		u := calculateCollectorURL(tc.license, tc.staging)
		c.Assert(u, Equals, tc.expectURL)
	}
}

func (s *ConfigSuite) TestCalculateDimensionalMetricURL(c *C) {
	testCases := []struct {
		name         string
		license      string
		collectorURL string
		staging      bool
		want         string
	}{
		{
			"Default URL, no region license, no collector URL",
			"0123456789012345678901234567890123456789",
			"",
			false,
			"https://metric-api.newrelic.com",
		},
		{
			"Staging URL, no region license, no collector URL",
			"0123456789012345678901234567890123456789",
			"",
			true,
			"https://staging-metric-api.newrelic.com",
		},
		{
			"Default URL, eu license region, no collector URL",
			"eu01xx6789012345678901234567890123456789",
			"",
			false,
			"https://metric-api.eu.newrelic.com",
		},
		{
			"Staging URL, eu license region, no collector URL",
			"eu01xx6789012345678901234567890123456789",
			"",
			true,
			"https://staging-metric-api.eu.newrelic.com",
		},
		{
			"Default URL, gov license region, no collector URL",
			"gov01xx6789012345678901234567890123456789",
			"",
			true,
			"https://gov-infra-api.newrelic.com",
		},
		{
			"From Collector URL",
			"gov01x6789012345678901234567890123456789",
			"https://metric-api.test",
			true,
			"https://metric-api.test",
		},
	}

	for _, tc := range testCases {
		u := calculateDimensionalMetricURL(tc.collectorURL, tc.license, tc.staging)
		c.Assert(u, Equals, tc.want)
	}
}

func (s *ConfigSuite) TestCalculateIdentityURL(c *C) {
	testcases := []struct {
		license   string
		expectURL string
		staging   bool
	}{
		// non-region license
		{license: "0123456789012345678901234567890123456789", expectURL: defaultIdentityURL, staging: false},
		// non-region license
		{license: "0123456789012345678901234567890123456789", expectURL: defaultIdentityStagingURL, staging: true},
		// four letter region
		{license: "eu01xx6789012345678901234567890123456789", expectURL: defaultIdentityURLEu, staging: false},
		// four letter region
		{license: "eu01xx6789012345678901234567890123456789", expectURL: defaultIdentityStagingURLEu, staging: true},
		// five letter region
		{license: "gov01x6789012345678901234567890123456789", expectURL: defaultIdentityURL, staging: false},
		// five letter region
		{license: "gov01x6789012345678901234567890123456789", expectURL: defaultIdentityStagingURL, staging: true},
	}

	for _, tc := range testcases {
		c.Assert(calculateIdentityURL(tc.license, tc.staging), Equals, tc.expectURL)
	}
}

func (s *ConfigSuite) TestCalculateCmdChannelURL(c *C) {
	testcases := []struct {
		license   string
		expectURL string
		staging   bool
	}{
		// non-region license
		{license: "0123456789012345678901234567890123456789", expectURL: defaultCmdChannelURL, staging: false},
		// non-region license
		{license: "0123456789012345678901234567890123456789", expectURL: defaultCmdChannelStagingURL, staging: true},
		// four letter region
		{license: "eu01xx6789012345678901234567890123456789", expectURL: defaultCmdChannelURLEu, staging: false},
		// four letter region
		{license: "eu01xx6789012345678901234567890123456789", expectURL: defaultCmdChannelStagingURLEu, staging: true},
		// five letter region
		{license: "gov01x6789012345678901234567890123456789", expectURL: defaultCmdChannelURL, staging: false},
		// five letter region
		{license: "gov01x6789012345678901234567890123456789", expectURL: defaultCmdChannelStagingURL, staging: true},
	}

	for _, tc := range testcases {
		c.Assert(calculateCmdChannelURL(tc.license, tc.staging), Equals, tc.expectURL)
	}
}

func TestLogInfo_Nil(t *testing.T) {
	var config *Config

	_, err := config.toLogInfo()
	assert.Error(t, err)
}

func TestLogInfo_Empty(t *testing.T) {
	var config Config

	_, err := config.toLogInfo()
	assert.NoError(t, err)
}

func TestLogInfo_New(t *testing.T) {
	var config = NewConfig()

	_, err := config.toLogInfo()
	assert.NoError(t, err)
}

func TestLogInfo_HidePrivate(t *testing.T) {
	var config = NewConfig()
	config.CollectorURL = "test"

	actual, err := config.toLogInfo()
	assert.NoError(t, err)

	_, exists := actual["collector_url"]
	assert.False(t, exists)
}

func TestLogInfo_Public(t *testing.T) {
	var config = NewConfig()
	config.Proxy = "test"

	actual, err := config.toLogInfo()
	assert.NoError(t, err)

	actualVal, exists := actual["proxy"]
	assert.True(t, exists)
	assert.Equal(t, "test", actualVal)
}

func TestLogInfo_Obfuscate(t *testing.T) {
	var config = NewConfig()
	config.License = "testabcd"

	actual, err := config.toLogInfo()
	assert.NoError(t, err)

	actualVal, exists := actual["license_key"]
	assert.True(t, exists)
	assert.Equal(t, "<HIDDEN>", actualVal)
}

func TestConfig_GetYamlAttribute(t *testing.T) {
	c := &Config{
		ConnectEnabled: false,
	}
	if err := c.SetBoolValueByYamlAttribute("connect_enabled", true); err != nil {
		t.Errorf("unable to update config value: %s", err)
	}
	assert.True(t, c.ConnectEnabled)
	assert.Error(t, c.SetBoolValueByYamlAttribute("no_a_value", false))
}

func (s *ConfigSuite) Test_ParseIncludeMatchingRules(c *C) {
	config := `
license_key: test
include_matching_metrics:
  process.name:
    - test
    - other-test
  process.executable:
    - regex "^some-process" 
`
	f, err := ioutil.TempFile("", "include_matching_rules_config_test")
	c.Assert(err, IsNil)
	_, err = f.WriteString(config)
	c.Assert(err, IsNil)
	_ = f.Close()
	defer func(f *os.File) {
		err = os.Remove(f.Name())
		c.Assert(err, IsNil)
	}(f)

	cfg, err := LoadConfig(f.Name())
	c.Assert(err, IsNil)
	c.Assert(cfg.IncludeMetricsMatchers, HasLen, 2)
	c.Assert(cfg.IncludeMetricsMatchers["process.name"], HasLen, 2)
	c.Assert(cfg.IncludeMetricsMatchers["process.executable"], HasLen, 1)
}

func Test_ParseIncludeMatchingRule_EnvVar(t *testing.T) {
	os.Setenv("NRIA_INCLUDE_MATCHING_METRICS", "process.name:\n - regex \"kube*\" \n")
	defer os.Unsetenv("NRIA_INCLUDE_MATCHING_METRICS")

	configStr := "license_key: abc123"
	f, err := ioutil.TempFile("", "yaml_config_test")
	assert.NoError(t, err)
	f.WriteString(configStr)
	f.Close()

	cfg, err := LoadConfig(f.Name())
	assert.NoError(t, err)
	expected := IncludeMetricsMap{"process.name": []string{"regex \"kube*\""}}
	assert.True(t, reflect.DeepEqual(cfg.IncludeMetricsMatchers, expected))
}

func TestLoadYamlConfig_withDatabindJSONVariables(t *testing.T) {
	yamlData := []byte(`
variables:
  var1:
    test:
      value: "10.0.2.2:8888"
staging: true
license_key: "xxx"
proxy: ${var1}
`)

	tmp, err := createTestFile(yamlData)
	require.NoError(t, err)
	defer os.Remove(tmp.Name())

	cfg, err := LoadConfig(tmp.Name())

	require.NoError(t, err)

	assert.True(t, cfg.Staging)
	assert.Equal(t, "xxx", cfg.License)
	assert.Equal(t, "10.0.2.2:8888", cfg.Proxy)
}

func TestLoadYamlConfig_withDatabindAndEnvVars(t *testing.T) {
	yamlData := []byte(`
variables:
  license:
    test:
      value: {{ SOME_LICENSE }}
license_key: ${license}
`)

	tmp, err := createTestFile(yamlData)
	require.NoError(t, err)
	defer os.Remove(tmp.Name())

	os.Setenv("SOME_LICENSE", "XXX")
	cfg, err := LoadConfig(tmp.Name())
	os.Unsetenv("SOME_LICENSE")

	require.NoError(t, err)
	assert.Equal(t, "XXX", cfg.License)
}

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

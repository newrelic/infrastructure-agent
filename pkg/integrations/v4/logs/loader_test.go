// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	dbDbPath = filepath.Join("/var/db/newrelic-infra/newrelic-integrations/logging", "fb.db")
	// inputs
	disabledTroubleshootCfg = config.Troubleshoot{}

	hostnameProvider = &mockHostnameResolver{}
	_, hostName, _   = hostnameProvider.Query()

	// Expected struct results
	fbCfgOutput = []FBCfgOutput{{
		Name:        "newrelic",
		Match:       "*",
		LicenseKey:  "license",
		SendMetrics: false,
	},
	}
	fbCfgEntityDecoration = FBCfgFilter{
		Name:  "record_modifier",
		Match: "*",
		Records: map[string]string{
			"entity.guid.INFRA": "FOOBAR",
			"plugin.type":       logRecordModifierSource,
			"hostname":          hostName,
		},
	}
	idnProvide = func() entity.Identity {
		return entity.Identity{
			ID:   13,
			GUID: "FOOBAR",
		}
	}
)

func TestCfgLoaderLoadAll(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.On("GetFeatureFlag", fflag.FlagFluentBitMetrics).Return(false, false)
	validContent := `
logs:
  - name: foo
    file: /file/path
`
	emptyCfg := FBCfg{}
	expectedCfg := FBCfg{
		Inputs: []FBCfgInput{
			{
				Name:           "tail",
				Tag:            "foo",
				Path:           "/file/path",
				BufferMaxSize:  "128k",
				MemBufferLimit: "16384k",
				DB:             dbDbPath,
				SkipLongLines:  "On",
				PathKey:        "filePath",
			},
		},
		Filters: []FBCfgFilter{
			{
				Name:  "record_modifier",
				Match: "foo",
				Records: map[string]string{
					"fb.input": "tail",
				},
			},
			fbCfgEntityDecoration,
		},
		Output: fbCfgOutput,
	}

	emptyDir, err := os.MkdirTemp("", "test-load-empty")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(emptyDir) })

	onlyEmptyCfg, err := os.MkdirTemp("", "test-load-non-empty")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(onlyEmptyCfg) })
	addFile(t, onlyEmptyCfg, "empty.yml", "")

	onlyValidCfg, err := os.MkdirTemp("", "test-load-content")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(onlyValidCfg) })
	addFile(t, onlyValidCfg, "valid.yml", validContent)

	onlyExampleFile, err := os.MkdirTemp("", "test-load-content")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(onlyExampleFile) })
	addFile(t, onlyExampleFile, "file.yml.example", validContent)

	exampleFileAndValidCfg, err := os.MkdirTemp("", "test-load-content")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(exampleFileAndValidCfg) })
	addFile(t, exampleFileAndValidCfg, "file.yml.example", validContent)
	addFile(t, exampleFileAndValidCfg, "valid.yml", validContent)

	tests := []struct {
		name     string
		folder   string
		wantCfg  FBCfg
		expectOK bool
	}{
		{"empty folder", emptyDir, emptyCfg, false},
		{"non-existing folder", "/some-non-existing-folder", emptyCfg, false},
		{"non-empty folder with YML file but no configs", onlyEmptyCfg, emptyCfg, false},
		{"folder with valid file", onlyValidCfg, expectedCfg, true},
		{"folder with only example (non-YML) files", onlyExampleFile, emptyCfg, false},
		{"folder with a valid file and example (non-YML) files", exampleFileAndValidCfg, expectedCfg, true},
		{"folder with valid file (verbose enabled)", onlyValidCfg, expectedCfg, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := newTestConf(tt.folder, disabledTroubleshootCfg, false)
			cfg, ok := NewFolderLoader(conf, idnProvide, hostnameProvider).LoadAll(ffRetriever)

			assert.Equal(t, tt.expectOK, ok)
			assert.Equal(t, tt.wantCfg, cfg)
		})
	}
}

func TestCfgLoader_LoadAll_VerboseEnabled(t *testing.T) {
	validContent := `
logs:
  - name: foo
    file: /file/path
`

	validFileFolder, err := os.MkdirTemp("", "test-load-content")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(validFileFolder) })
	addFile(t, validFileFolder, "file.yml", validContent)

	expectedCfg := FBCfg{
		Inputs: []FBCfgInput{
			{
				Name:           "tail",
				Tag:            "foo",
				Path:           "/file/path",
				BufferMaxSize:  "128k",
				MemBufferLimit: "16384k",
				DB:             dbDbPath,
				SkipLongLines:  "On",
				PathKey:        "filePath",
			},
		},
		Filters: []FBCfgFilter{
			{
				Name:  "record_modifier",
				Match: "foo",
				Records: map[string]string{
					"fb.input": "tail",
				},
			},
			fbCfgEntityDecoration,
		},
		Output: []FBCfgOutput{{
			Name:        "newrelic",
			Match:       "*",
			LicenseKey:  "license",
			SendMetrics: true,
		}},
	}

	conf := newTestConf(validFileFolder, disabledTroubleshootCfg, true)
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.On("GetFeatureFlag", fflag.FlagFluentBitMetrics).Return(false, false)
	cfg, ok := NewFolderLoader(conf, idnProvide, hostnameProvider).LoadAll(ffRetriever)
	assert.Equal(t, true, ok)
	assert.Equal(t, expectedCfg, cfg)
}

func TestCfgLoaderTroubleshootDisabled(t *testing.T) {
	disabledTroubleshootCfg := config.NewTroubleshootCfg(false, false, "")
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.On("GetFeatureFlag", fflag.FlagFluentBitMetrics).Return(false, false)
	_, ok := NewFolderLoader(newTestConf("", disabledTroubleshootCfg, false), idnProvide, hostnameProvider).LoadAll(ffRetriever)
	assert.False(t, ok, "should return ok=false when there is no logging configuration directory and troubleshoot is disabled")
}

func TestCfgLoaderTroubleshootNoLogFile(t *testing.T) {
	troublesCfg := config.NewTroubleshootCfg(true, false, "")
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.On("GetFeatureFlag", fflag.FlagFluentBitMetrics).Return(false, false)
	cfg, ok := NewFolderLoader(newTestConf("", troublesCfg, false), idnProvide, hostnameProvider).LoadAll(ffRetriever)
	assert.True(t, ok, "Enabling troubleshoot with no logging configurations should start the log forwarder")
	assert.Equal(t, FBCfg{
		Inputs: []FBCfgInput{
			{
				Name:           "systemd",
				Tag:            fluentBitTagTroubleshoot,
				DB:             dbDbPath,
				Systemd_Filter: "_SYSTEMD_UNIT=newrelic-infra.service",
			},
		},
		Filters: []FBCfgFilter{
			{
				Name:  "record_modifier",
				Match: fluentBitTagTroubleshoot,
				Records: map[string]string{
					"fb.input": "systemd",
				},
			},
			fbCfgEntityDecoration,
		},
		Output: fbCfgOutput,
	}, cfg)
}

func TestCfgLoader_LoadAll_TroubleshootLogFile(t *testing.T) {
	troublesCfg := config.NewTroubleshootCfg(true, true, "/agent_log_file")
	ffRetriever := &feature_flags.FeatureFlagRetrieverMock{}
	ffRetriever.On("GetFeatureFlag", fflag.FlagFluentBitMetrics).Return(false, false)
	cfg, ok := NewFolderLoader(newTestConf("", troublesCfg, false), idnProvide, hostnameProvider).LoadAll(ffRetriever)
	assert.True(t, ok, "Enabling troubleshoot with a log file should start the log forwarder")
	assert.Equal(t, FBCfg{
		Inputs: []FBCfgInput{
			{
				Name:           "tail",
				DB:             dbDbPath,
				Path:           "/agent_log_file",
				BufferMaxSize:  "128k",
				MemBufferLimit: "16384k",
				SkipLongLines:  "On",
				PathKey:        "filePath",
				Tag:            fluentBitTagTroubleshoot,
			},
		},
		Filters: []FBCfgFilter{
			{
				Name:  "record_modifier",
				Match: fluentBitTagTroubleshoot,
				Records: map[string]string{
					"fb.input": "tail",
				},
			},
			fbCfgEntityDecoration,
		},
		Output: fbCfgOutput,
	}, cfg)
}

func TestCfgLoader_parseYAML(t *testing.T) {
	ymlWithFile := []byte(`
logs:
  - name: foo
    file: /file/path
`)
	structWithFile := LogsCfg{
		{
			Name: "foo",
			File: "/file/path",
		},
	}

	ymlWithSystemd := []byte(`
logs:
  - name: bar
    systemd: bar-svc
    pattern: "regex"
`)
	structWithSystemd := LogsCfg{
		{
			Name:    "bar",
			Systemd: "bar-svc",
			Pattern: "regex",
		},
	}

	ymlInvalid := []byte(`
nooo:
  - name: fuuu
`)
	ymlPartiallyInvalid := []byte(`
logs:
  - name: fuuu
    wrong: field
`)
	ymlWithAttributes := []byte(`
logs:
  - name: attributed-file
    file: '/foo/bar.log'
    attributes:
      key1: value1
      key2: value2
      key3 with space: value3 with space
`)
	structWithAttributes := LogsCfg{
		{
			Name: "attributed-file",
			File: "/foo/bar.log",
			Attributes: map[string]string{
				"key1":            "value1",
				"key2":            "value2",
				"key3 with space": "value3 with space",
			},
		},
	}

	ymlWithTcpSyslog := []byte(`
logs:
  - name: syslog-tcp-test
    syslog:
      uri: tcp://0.0.0.0:5140
      parser: syslog-rfc5424
`)
	structWithTcpSyslog := LogsCfg{
		{
			Name: "syslog-tcp-test",
			Syslog: &LogSyslogCfg{
				URI:    "tcp://0.0.0.0:5140",
				Parser: "syslog-rfc5424",
			},
		},
	}

	ymlWithUdpSyslog := []byte(`
logs:
  - name: syslog-udp-test
    syslog:
      uri: udp://0.0.0.0:5140
      parser: syslog-rfc5424
`)
	structWithUdpSyslog := LogsCfg{
		{
			Name: "syslog-udp-test",
			Syslog: &LogSyslogCfg{
				URI:    "udp://0.0.0.0:5140",
				Parser: "syslog-rfc5424",
			},
		},
	}

	ymlWithUnixTcpSyslog := []byte(`
logs:
  - name: syslog-unix-tcp-test
    syslog:
      uri: unix_tcp:///var/tcp-socket-test
      parser: syslog-rfc3164
      unix_permissions: 0644
`)
	structWithUnixTcpSyslog := LogsCfg{
		{
			Name: "syslog-unix-tcp-test",
			Syslog: &LogSyslogCfg{
				URI: "unix_tcp:///var/tcp-socket-test", Parser: "syslog-rfc3164",
				UnixPermissions: "0644",
			},
		},
	}

	ymlWithUnixUdpSyslog := []byte(`
logs:
  - name: syslog-unix-udp-test
    syslog:
      uri: unix_udp:///var/udp-socket-test
      parser: syslog-rfc3164
      unix_permissions: 0644
    max_line_kb: 64
`)
	structWithUnixUdpSyslog := LogsCfg{
		{
			Name: "syslog-unix-udp-test",
			Syslog: &LogSyslogCfg{
				URI:             "unix_udp:///var/udp-socket-test",
				Parser:          "syslog-rfc3164",
				UnixPermissions: "0644",
			},
			MaxLineKb: 64,
		},
	}

	ymlWithTcp := []byte(`
logs:
  - name: tcp-test
    tcp:
      uri: tcp://0.0.0.0:1234
      format: none
      separator: \n
`)
	structWithTcp := LogsCfg{
		{
			Name: "tcp-test",
			Tcp: &LogTcpCfg{
				Uri:       "tcp://0.0.0.0:1234",
				Format:    "none",
				Separator: "\\n",
			},
		},
	}

	ymlWithExternalFBCfg := []byte(`
logs:
  - name: fb-test
    fluentbit:
      config_file: /path/to/fb/config
      parsers_file: /path/to/fb/parsers
`)
	structWithExternalFBCfg := LogsCfg{
		{
			Name: "fb-test",
			Fluentbit: &LogExternalFBCfg{
				CfgPath:     "/path/to/fb/config",
				ParsersPath: "/path/to/fb/parsers",
			},
		},
	}

	tests := []struct {
		name     string
		contents []byte
		wantC    LogsCfg
		wantErr  error
	}{
		{"empty file", []byte{}, nil, nil},
		{"input with file", ymlWithFile, structWithFile, nil},
		{"input with systemd and grep", ymlWithSystemd, structWithSystemd, nil},
		{"input invalid", ymlInvalid, nil, nil},
		{"input partially invalid", ymlPartiallyInvalid, nil, nil},
		{"file with attributes", ymlWithAttributes, structWithAttributes, nil},
		{"syslog tcp", ymlWithTcpSyslog, structWithTcpSyslog, nil},
		{"syslog udp", ymlWithUdpSyslog, structWithUdpSyslog, nil},
		{"syslog tcp_unix", ymlWithUnixTcpSyslog, structWithUnixTcpSyslog, nil},
		{"syslog udp_unix", ymlWithUnixUdpSyslog, structWithUnixUdpSyslog, nil},
		{"input tcp", ymlWithTcp, structWithTcp, nil},
		{"external FB config and parsers", ymlWithExternalFBCfg, structWithExternalFBCfg, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// SUT
			gotC, err := NewFolderLoader(newTestConf("", disabledTroubleshootCfg, false), idnProvide, hostnameProvider).parseYAML(tt.contents)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.wantC, gotC)
		})
	}
}
func newTestConf(folder string, troubleCfg config.Troubleshoot, fluentBitVerbose bool) config.LogForward {
	cfg := &config.Config{
		LoggingHomeDir:    "/var/db/newrelic-infra/newrelic-integrations/logging",
		LoggingConfigsDir: folder,
		License:           "license",
	}
	if fluentBitVerbose {
		cfg.Log.Level = config.LogLevelTrace
		cfg.Log.IncludeFilters = map[string][]interface{}{"traces": {"supervisor"}}
	}
	return config.NewLogForward(cfg, troubleCfg)
}

type mockHostnameResolver struct {
}

func (m *mockHostnameResolver) Query() (full, short string, err error) {
	return "full", "ubuntu", nil
}

func (m *mockHostnameResolver) Long() string {
	return "full"
}

func addFile(t *testing.T, dir, name, contents string) {
	filePath := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(filePath, []byte(contents), 0666))
}

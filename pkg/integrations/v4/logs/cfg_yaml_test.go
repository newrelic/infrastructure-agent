// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

//nolint:exhaustruct,funlen
func TestFBCfgFormatYAML(t *testing.T) {
	t.Parallel()

	fbCfg := FBCfg{
		Inputs: []FBCfgInput{
			{
				Name: "tail", Path: `C:\Program Files\app\*.log`, BufferMaxSize: "128k", MemBufferLimit: "16384k",
				SkipLongLines: "On", PathKey: "filePath", Tag: "my-file", DB: "/var/db/fb.db",
				MultilineParser: "java", UnicodeEncoding: "UTF-16LE",
			},
			{Name: "systemd", Systemd_Filter: "_SYSTEMD_UNIT=nginx.service", Tag: "nginx", DB: "/var/db/fb.db"},
			{
				Name: "syslog", Tag: "sl", SyslogMode: "tcp", SyslogListen: "0.0.0.0", SyslogPort: 5140,
				SyslogParser: "rfc3164", BufferMaxSize: "128k",
			},
			{
				Name: "syslog", Tag: "slu", SyslogMode: "unix_udp", SyslogUnixPath: "/var/sock",
				SyslogUnixPermissions: "0644", SyslogParser: "rfc5424", BufferChunkSize: "128k",
			},
			{
				Name: "tcp", Tag: "t", TcpListen: "0.0.0.0", TcpPort: 2000, TcpFormat: "none",
				TcpSeparator: `\n`, TcpBufferSize: 128,
			},
			{Name: "winlog", Channels: "Security,Application", Tag: "win", DB: `C:\fb.db`, UseANSI: "true"},
		},
		Filters: []FBCfgFilter{
			{
				Name: "record_modifier", Match: "my-file",
				Records: map[string]string{"fb.input": "tail", "team name": "core: infra #1"},
			},
			{Name: "grep", Match: "my-file", Regex: "log ^#.*: *error"},
			{Name: "lua", Match: "win", Script: `C:\Temp\nr_fb_lua_filter123`, Call: "eventIdFilter"},
			{
				Name: "modify", Match: "win",
				Modifiers: map[string]string{"Message": "message", "EventType": "WinEventType"},
			},
		},
		Output: FBCfgOutput{
			Name: "newrelic", Match: "*", LicenseKey: "abc", Endpoint: "https://log-api.eu.newrelic.com/log/v1",
			Proxy: "http://p:3128", IgnoreSystemProxy: true, CABundleFile: "/ca.pem", CABundleDir: "/ca",
			Retry_Limit: "5", HTTPClientTimeout: "15", SendMetrics: true,
		},
		ExternalCfg: FBCfgExternal{CfgFilePath: "/etc/fb/custom.yaml", ParsersFilePath: "/etc/fb/parsers.conf"},
	}

	expected := `pipeline:
  inputs:
    - name: tail
      path: C:\Program Files\app\*.log
      buffer_max_size: 128k
      mem_buf_limit: 16384k
      skip_long_lines: On
      multiline.parser: java
      unicode.encoding: UTF-16LE
      path_key: filePath
      tag: my-file
      db: /var/db/fb.db
    - name: systemd
      tag: nginx
      db: /var/db/fb.db
      systemd_filter: _SYSTEMD_UNIT=nginx.service
    - name: syslog
      buffer_max_size: 128k
      tag: sl
      mode: tcp
      listen: 0.0.0.0
      port: 5140
      parser: rfc3164
    - name: syslog
      buffer_chunk_size: 128k
      tag: slu
      mode: unix_udp
      parser: rfc5424
      path: /var/sock
      unix_perm: "0644"
    - name: tcp
      tag: t
      listen: 0.0.0.0
      port: 2000
      format: none
      separator: \n
      buffer_size: 128
    - name: winlog
      tag: win
      db: C:\fb.db
      channels: Security,Application
      use_ansi: "true"
  filters:
    - name: record_modifier
      match: my-file
      record:
        - '"fb.input" "tail"'
        - '"team name" "core: infra #1"'
    - name: grep
      match: my-file
      regex: 'log ^#.*: *error'
    - name: lua
      match: win
      script: C:\Temp\nr_fb_lua_filter123
      call: eventIdFilter
    - name: modify
      match: win
      rename:
        - EventType WinEventType
        - Message message
  outputs:
    - name: newrelic
      match: '*'
      licensekey: ${NR_LICENSE_KEY_ENV_VAR}
      endpoint: https://log-api.eu.newrelic.com/log/v1
      proxy: http://p:3128
      ignoresystemproxy: "true"
      cabundlefile: /ca.pem
      cabundledir: /ca
      validateproxycerts: "false"
      retry_limit: "5"
      httpclienttimeout: "15"
      sendmetrics: "true"
includes:
  - /etc/fb/custom.yaml
`

	result, extCfg, err := fbCfg.FormatYAML()
	require.NoError(t, err)
	assert.Equal(t, fbCfg.ExternalCfg, extCfg)
	assert.Equal(t, expected, result)

	// values must survive a YAML round trip unchanged (quoting of paths, regexes, '*', ...)
	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(result), &parsed))
}

//nolint:exhaustruct
func TestFBCfgFormatYAML_IncludesAreWrittenLast(t *testing.T) {
	t.Parallel()

	// Fluent Bit merges includes in document order. "includes" must follow "pipeline" so external
	// filters run after the agent filters, like the classic "@INCLUDE" at the end of the file.
	fbCfg := FBCfg{
		Filters:     []FBCfgFilter{{Name: "record_modifier", Match: "*", Records: map[string]string{"a": "b"}}},
		Output:      FBCfgOutput{Name: "newrelic", Match: "*", ValidateCerts: true},
		ExternalCfg: FBCfgExternal{CfgFilePath: "/etc/fb/custom.yaml"},
	}

	result, _, err := fbCfg.FormatYAML()
	require.NoError(t, err)

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(result), &doc))
	root := doc.Content[0]
	require.Len(t, root.Content, 4)
	assert.Equal(t, "pipeline", root.Content[0].Value)
	assert.Equal(t, "includes", root.Content[2].Value)
}

//nolint:exhaustruct
func TestFBCfgFormatYAML_OmitsEmptySections(t *testing.T) {
	t.Parallel()

	fbCfg := FBCfg{
		Inputs: []FBCfgInput{{Name: "tail", Path: "/var/log/a.log", Tag: "a"}},
		Output: FBCfgOutput{Name: "newrelic", Match: "*", ValidateCerts: true},
	}

	expected := `pipeline:
  inputs:
    - name: tail
      path: /var/log/a.log
      tag: a
  outputs:
    - name: newrelic
      match: '*'
`

	result, extCfg, err := fbCfg.FormatYAML()
	require.NoError(t, err)
	assert.Empty(t, extCfg)
	assert.Equal(t, expected, result)
}

//nolint:exhaustruct
func TestFBCfgFormatAs(t *testing.T) {
	t.Parallel()

	fbCfg := FBCfg{
		Inputs: []FBCfgInput{{Name: "tail", Path: "/var/log/a.log", Tag: "a"}},
		Output: FBCfgOutput{Name: "newrelic", Match: "*", ValidateCerts: true},
	}

	classic, _, err := fbCfg.Format()
	require.NoError(t, err)
	yamlCfg, _, err := fbCfg.FormatYAML()
	require.NoError(t, err)

	cases := []struct {
		name     string
		format   FBConfigFormat
		expected string
	}{
		{"classic", FBConfigFormatClassic, classic},
		{"yaml", FBConfigFormatYAML, yamlCfg},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, _, err := fbCfg.FormatAs(tc.format)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

//nolint:exhaustruct
func TestFBCfgExternal_ConfigFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path     string
		expected FBConfigFormat
	}{
		{"", ""},
		{"/etc/fb/custom.conf", FBConfigFormatClassic},
		{"/etc/fb/custom", FBConfigFormatClassic},
		{"/etc/fb/custom.yaml", FBConfigFormatYAML},
		{`C:\fb\custom.YML`, FBConfigFormatYAML},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, FBCfgExternal{CfgFilePath: tc.path}.ConfigFormat())
		})
	}
}

func TestFBConfigFormat_FileExtension(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ".yaml", FBConfigFormatYAML.FileExtension())
	assert.Equal(t, ".conf", FBConfigFormatClassic.FileExtension())
}

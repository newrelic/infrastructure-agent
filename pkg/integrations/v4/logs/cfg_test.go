// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFBConf(t *testing.T) {
	logFwdCfg := &config.LogForward{
		HomeDir:   "/var/db/newrelic-infra/newrelic-integrations/logging",
		License:   "licenseKey",
		IsStaging: false,
		ProxyCfg: config.LogForwardProxy{
			IgnoreSystemProxy: true,
			Proxy:             "https://https-proxy:3129",
			CABundleFile:      "/cabundles/proxycert.pem",
			CABundleDir:       "/cabundles",
			ValidateCerts:     true,
		},
	}

	parserEntityBlock := FBCfgParser{
		Name:  "record_modifier",
		Match: "*",
		Records: map[string]string{
			"entity.guid": "0",
			"plugin.type": "nri-agent",
			"hostname":    "",
		},
	}
	inputRecordModifier := func(i string, m string) FBCfgParser {
		return FBCfgParser{
			Name:  "record_modifier",
			Match: m,
			Records: map[string]string{
				"fb.input": i,
			},
		}
	}
	outputBlock := FBCfgOutput{
		Name:              "newrelic",
		Match:             "*",
		LicenseKey:        "licenseKey",
		IgnoreSystemProxy: true,
		Proxy:             "https://https-proxy:3129",
		CABundleFile:      "/cabundles/proxycert.pem",
		CABundleDir:       "/cabundles",
		ValidateCerts:     true,
	}

	tests := []struct {
		name   string
		ohiCfg LogsCfg
		want   FBCfg
	}{
		{"empty", LogsCfg{},
			FBCfg{
				Inputs:  []FBCfgInput{},
				Parsers: []FBCfgParser{},
			}},
		{"single input", LogsCfg{
			{
				Name: "log-file",
				File: "file.path",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tail",
					Tag:           "log-file",
					DB:            dbDbPath,
					Path:          "file.path",
					BufferMaxSize: "128k",
					SkipLongLines: "On",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tail", "log-file"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input file + parser", LogsCfg{
			{
				Name:    "log-file",
				File:    "file.path",
				Pattern: "foo",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tail",
					Tag:           "log-file",
					DB:            dbDbPath,
					Path:          "file.path",
					BufferMaxSize: "128k",
					SkipLongLines: "On",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tail", "log-file"),
				{
					Name:  "grep",
					Match: "log-file",
					Regex: "log foo",
				},
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input systemd + parser", LogsCfg{
			{
				Name:    "some_system",
				Systemd: "service_name",
				Pattern: "foo",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:           "systemd",
					Tag:            "some_system",
					DB:             dbDbPath,
					Systemd_Filter: "_SYSTEMD_UNIT=service_name.service",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("systemd", "some_system"),
				{
					Name:  "grep",
					Match: "some_system",
					Regex: "MESSAGE foo",
				},
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input folder with files", LogsCfg{
			{
				Name:      "some-folder",
				Folder:    "/path/to/folder",
				MaxLineKb: 32,
				Pattern:   "foo",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name: "tail",
					Tag:  "some-folder",
					DB:   dbDbPath,
					// filepath.Join used here as the test outputs the result as \path\to\folder\* when executing on Windows
					Path:          filepath.Join("/path/to/folder", "*"),
					BufferMaxSize: "32k",
					SkipLongLines: "On",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tail", "some-folder"),
				{
					Name:  "grep",
					Match: "some-folder",
					Regex: "log foo",
				},
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input win-eventlog + parser", LogsCfg{
			{
				Name:     "win-eventlog",
				EventLog: "Application",
				Pattern:  "something",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:     "winlog",
					Tag:      "win-eventlog",
					DB:       dbDbPath,
					Channels: "Application",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("winlog", "win-eventlog"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"single file with attributes", LogsCfg{
			{
				Name: "one-file",
				File: "/foo/file.foo",
				Attributes: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tail",
					Tag:           "one-file",
					DB:            dbDbPath,
					Path:          "/foo/file.foo",
					BufferMaxSize: "128k",
					SkipLongLines: "On",
				},
			},
			Parsers: []FBCfgParser{
				{
					Name:  "record_modifier",
					Match: "one-file",
					Records: map[string]string{
						"fb.input": "tail",
						"key1":     "value1",
						"key2":     "value2",
					},
				},
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"file with reserved attribute names", LogsCfg{
			{
				Name: "reserved-test",
				File: "/foo/file.foo",
				Attributes: map[string]string{
					"valid":       "value",
					"entity.guid": "should-be-ignored",
				},
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tail",
					Tag:           "reserved-test",
					DB:            dbDbPath,
					Path:          "/foo/file.foo",
					BufferMaxSize: "128k",
					SkipLongLines: "On",
				},
			},
			Parsers: []FBCfgParser{
				{
					Name:  "record_modifier",
					Match: "reserved-test",
					Records: map[string]string{
						"fb.input": "tail",
						"valid":    "value",
					},
				},
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input syslog tcp any interface", LogsCfg{
			{
				Name: "syslog-tcp-test",
				Syslog: &LogSyslogCfg{
					URI:    "tcp://0.0.0.0:5140",
					Parser: "syslog-rfc5424",
				},
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "syslog",
					Tag:           "syslog-tcp-test",
					SyslogMode:    "tcp",
					SyslogListen:  "0.0.0.0",
					SyslogPort:    5140,
					SyslogParser:  "syslog-rfc5424",
					BufferMaxSize: "128k",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("syslog", "syslog-tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input syslog tcp localhost", LogsCfg{
			{
				Name: "syslog-tcp-test",
				Syslog: &LogSyslogCfg{
					URI:    "tcp://127.0.0.1:5140",
					Parser: "syslog-rfc5424",
				},
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "syslog",
					Tag:           "syslog-tcp-test",
					SyslogMode:    "tcp",
					SyslogListen:  "127.0.0.1",
					SyslogPort:    5140,
					SyslogParser:  "syslog-rfc5424",
					BufferMaxSize: "128k",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("syslog", "syslog-tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input syslog tcp specific interface", LogsCfg{
			{
				Name: "syslog-tcp-test",
				Syslog: &LogSyslogCfg{
					URI:    "tcp://192.168.1.135:5140",
					Parser: "syslog-rfc5424",
				},
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "syslog",
					Tag:           "syslog-tcp-test",
					SyslogMode:    "tcp",
					SyslogListen:  "192.168.1.135",
					SyslogPort:    5140,
					SyslogParser:  "syslog-rfc5424",
					BufferMaxSize: "128k",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("syslog", "syslog-tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input syslog udp", LogsCfg{
			{
				Name: "syslog-udp-test",
				Syslog: &LogSyslogCfg{
					URI:    "udp://0.0.0.0:5140",
					Parser: "syslog-rfc5424",
				},
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:            "syslog",
					Tag:             "syslog-udp-test",
					SyslogMode:      "udp",
					SyslogListen:    "0.0.0.0",
					SyslogPort:      5140,
					SyslogParser:    "syslog-rfc5424",
					BufferChunkSize: "128k",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("syslog", "syslog-udp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input syslog tcp_unix", LogsCfg{
			{
				Name: "syslog-unix-tcp-test",
				Syslog: &LogSyslogCfg{
					URI:             "unix_tcp:///var/tcp-socket-test",
					Parser:          "syslog-rfc3164",
					UnixPermissions: "0644",
				},
				MaxLineKb: 640,
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:                  "syslog",
					Tag:                   "syslog-unix-tcp-test",
					SyslogMode:            "unix_tcp",
					SyslogUnixPath:        "/var/tcp-socket-test",
					SyslogUnixPermissions: "0644",
					SyslogParser:          "syslog-rfc3164",
					BufferMaxSize:         "640k",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("syslog", "syslog-unix-tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input syslog udp_unix", LogsCfg{
			{
				Name: "syslog-unix-udp-test",
				Syslog: &LogSyslogCfg{
					URI: "unix_udp:///var/udp-socket-test",
					// parser omitted intentionally, it should be rfc3164 by default
					UnixPermissions: "0644",
				},
				MaxLineKb: 64,
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:                  "syslog",
					Tag:                   "syslog-unix-udp-test",
					SyslogMode:            "unix_udp",
					SyslogUnixPath:        "/var/udp-socket-test",
					SyslogUnixPermissions: "0644",
					SyslogParser:          "rfc3164",
					BufferChunkSize:       "64k",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("syslog", "syslog-unix-udp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input tcp any interface", LogsCfg{
			{
				Name: "tcp-test",
				Tcp: &LogTcpCfg{
					Uri:       "tcp://0.0.0.0:2222",
					Format:    "none",
					Separator: `\\n`,
				},
				MaxLineKb: 64,
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tcp",
					Tag:           "tcp-test",
					TcpListen:     "0.0.0.0",
					TcpPort:       2222,
					TcpFormat:     "none",
					TcpSeparator:  `\n`,
					TcpBufferSize: 64,
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tcp", "tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input tcp localhost", LogsCfg{
			{
				Name: "tcp-test",
				Tcp: &LogTcpCfg{
					Uri:       "tcp://127.0.0.1:2222",
					Format:    "none",
					Separator: `\\n`,
				},
				MaxLineKb: 64,
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tcp",
					Tag:           "tcp-test",
					TcpListen:     "127.0.0.1",
					TcpPort:       2222,
					TcpFormat:     "none",
					TcpSeparator:  `\n`,
					TcpBufferSize: 64,
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tcp", "tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input tcp specific interface", LogsCfg{
			{
				Name: "tcp-test",
				Tcp: &LogTcpCfg{
					Uri:       "tcp://192.168.1.135:2222",
					Format:    "none",
					Separator: `\\n`,
				},
				MaxLineKb: 64,
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tcp",
					Tag:           "tcp-test",
					TcpListen:     "192.168.1.135",
					TcpPort:       2222,
					TcpFormat:     "none",
					TcpSeparator:  `\n`,
					TcpBufferSize: 64,
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tcp", "tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"existing Fluent Bit configuration", LogsCfg{
			{
				Name: "fb-test",
				Fluentbit: &LogExternalFBCfg{
					CfgPath:     "/path/to/config/file",
					ParsersPath: "/path/to/parsers/file",
				},
			},
			{
				// This service is added to test a bug that had been accidentally introduced
				Name:    "dummy_system",
				Systemd: "service_name",
				Pattern: "foo",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:           "systemd",
					Tag:            "dummy_system",
					DB:             dbDbPath,
					Systemd_Filter: "_SYSTEMD_UNIT=service_name.service",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("systemd", "dummy_system"),
				{
					Name:  "grep",
					Match: "dummy_system",
					Regex: "MESSAGE foo",
				},
				parserEntityBlock,
			},
			Output: outputBlock,
			ExternalCfg: FBCfgExternal{
				CfgFilePath:     "/path/to/config/file",
				ParsersFilePath: "/path/to/parsers/file",
			},
		}},
		{"existing Fluent Bit configuration, duplicated", LogsCfg{
			{
				Name: "fb-test",
				Fluentbit: &LogExternalFBCfg{
					CfgPath:     "/path/to/config/file",
					ParsersPath: "/path/to/parsers/file",
				},
			},
			{
				Name: "fb-test-should-be-ignored",
				Fluentbit: &LogExternalFBCfg{
					CfgPath:     "/path/to/config/file-should-be-ignored",
					ParsersPath: "/path/to/parsers/file-should-be-ignored",
				},
			},
		}, FBCfg{
			Inputs: []FBCfgInput{},
			Parsers: []FBCfgParser{
				parserEntityBlock,
			},
			Output: outputBlock,
			ExternalCfg: FBCfgExternal{
				CfgFilePath:     "/path/to/config/file",
				ParsersFilePath: "/path/to/parsers/file",
			},
		}},
		{"input syslog tcp any interface with Pattern", LogsCfg{
			{
				Name: "syslog-tcp-test",
				Syslog: &LogSyslogCfg{
					URI:    "tcp://0.0.0.0:5140",
					Parser: "syslog-rfc5424",
				},
				Pattern: "foo",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "syslog",
					Tag:           "syslog-tcp-test",
					SyslogMode:    "tcp",
					SyslogListen:  "0.0.0.0",
					SyslogPort:    5140,
					SyslogParser:  "syslog-rfc5424",
					BufferMaxSize: "128k",
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("syslog", "syslog-tcp-test"),
				{
					Name:  "grep",
					Match: "syslog-tcp-test",
					Regex: "message foo",
				},
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input tcp any interface with Pattern", LogsCfg{
			{
				Name: "tcp-test",
				Tcp: &LogTcpCfg{
					Uri:       "tcp://0.0.0.0:2222",
					Format:    "none",
					Separator: `\\n`,
				},
				MaxLineKb: 64,
				Pattern:   "foo",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tcp",
					Tag:           "tcp-test",
					TcpListen:     "0.0.0.0",
					TcpPort:       2222,
					TcpFormat:     "none",
					TcpSeparator:  `\n`,
					TcpBufferSize: 64,
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tcp", "tcp-test"),
				{
					Name:  "grep",
					Match: "tcp-test",
					Regex: "log foo",
				},
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
		{"input tcp any interface with Pattern in json format", LogsCfg{
			{
				Name: "tcp-test",
				Tcp: &LogTcpCfg{
					Uri:       "tcp://0.0.0.0:2222",
					Format:    "json",
					Separator: `\\n`,
				},
				MaxLineKb: 64,
				Pattern:   "foo",
			},
		}, FBCfg{
			Inputs: []FBCfgInput{
				{
					Name:          "tcp",
					Tag:           "tcp-test",
					TcpListen:     "0.0.0.0",
					TcpPort:       2222,
					TcpFormat:     "json",
					TcpBufferSize: 64,
				},
			},
			Parsers: []FBCfgParser{
				inputRecordModifier("tcp", "tcp-test"),
				parserEntityBlock,
			},
			Output: outputBlock,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fbConf, err := NewFBConf(tt.ohiCfg, logFwdCfg, "0", "")
			assert.NoError(t, err)
			assert.Equal(t, tt.want, fbConf)
		})
	}
}

func TestFBCfgFormat(t *testing.T) {
	expected := `
[INPUT]
    Name tail
    Path /path/to/folder/*
    Buffer_Max_Size 32k
    Skip_Long_Lines On
    Tag  some-folder
    DB   fb.db

[INPUT]
    Name systemd
    Tag  some-service
    DB   fb.db
    Systemd_Filter _SYSTEMD_UNIT=service-test.service

[INPUT]
    Name syslog
    Buffer_Max_Size 128k
    Tag  syslog-tcp-test
    Mode tcp
    Listen 0.0.0.0
    Port 5140
    Parser syslog-rfc5424

[INPUT]
    Name syslog
    Buffer_Chunk_Size 64k
    Tag  syslog-unix-udp-test
    Mode unix_udp
    Parser syslog-rfc3164
    Path /var/udp-socket-test
    Unix_Perm 0644

[INPUT]
    Name tcp
    Tag  tcp-test
    Listen 0.0.0.0
    Port 1234
    Format none
    Separator \n
    Buffer_Size 32

[FILTER]
    Name  grep
    Match some-folder
    Regex log foo

[FILTER]
    Name  record_modifier
    Match some-folder
    Record fb.input tail
    Record key1 value1
    Record key2 value2

[FILTER]
    Name  record_modifier
    Match *
    Record entity.guid testGUID
    Record fb.source nri-agent

[OUTPUT]
    Name                newrelic
    Match               *
    licenseKey          licenseKey
    validateProxyCerts  false

@INCLUDE /path/to/fb/config
`

	fbCfg := FBCfg{
		Inputs: []FBCfgInput{
			{
				Name:          "tail",
				Tag:           "some-folder",
				DB:            "fb.db",
				Path:          "/path/to/folder/*",
				BufferMaxSize: "32k",
				SkipLongLines: "On",
			},
			{
				Name:           "systemd",
				Tag:            "some-service",
				DB:             "fb.db",
				Systemd_Filter: "_SYSTEMD_UNIT=service-test.service",
			},
			{
				Name:          "syslog",
				Tag:           "syslog-tcp-test",
				SyslogMode:    "tcp",
				SyslogListen:  "0.0.0.0",
				SyslogPort:    5140,
				SyslogParser:  "syslog-rfc5424",
				BufferMaxSize: "128k",
			},
			{
				Name:                  "syslog",
				Tag:                   "syslog-unix-udp-test",
				SyslogMode:            "unix_udp",
				SyslogUnixPath:        "/var/udp-socket-test",
				SyslogUnixPermissions: "0644",
				SyslogParser:          "syslog-rfc3164",
				BufferChunkSize:       "64k",
			},
			{
				Name:          "tcp",
				Tag:           "tcp-test",
				TcpListen:     "0.0.0.0",
				TcpPort:       1234,
				TcpFormat:     "none",
				TcpSeparator:  "\\n",
				TcpBufferSize: 32,
			},
		},
		Parsers: []FBCfgParser{
			{
				Name:  "grep",
				Match: "some-folder",
				Regex: "log foo",
			},
			{
				Name:  "record_modifier",
				Match: "some-folder",
				Records: map[string]string{
					"fb.input": "tail",
					"key1":     "value1",
					"key2":     "value2",
				},
			},
			{
				Name:  "record_modifier",
				Match: "*",
				Records: map[string]string{
					"entity.guid": "testGUID",
					"fb.source":   "nri-agent",
				},
			},
		},
		Output: FBCfgOutput{
			Name:       "newrelic",
			Match:      "*",
			LicenseKey: "licenseKey",
		},
		ExternalCfg: FBCfgExternal{
			CfgFilePath:     "/path/to/fb/config",
			ParsersFilePath: "/path/to/fb/parsers",
		},
	}

	result, extCfg, err := fbCfg.Format()
	assert.Empty(t, err)
	assert.Equal(t, "/path/to/fb/parsers", extCfg.ParsersFilePath)
	assert.Equal(t, expected, result)
}

func TestFBCfgFormatWithHostname(t *testing.T) {
	expected := `
[INPUT]
    Name tail
    Path file.foo
    Buffer_Max_Size 32k
    Skip_Long_Lines On
    Tag  some-file
    DB   fb.db

[FILTER]
    Name  grep
    Match some-file
    Regex log foo

[FILTER]
    Name  record_modifier
    Match *
    Record entity.guid testGUID
    Record hostname ubuntu
    Record plugin.type nri-agent

[OUTPUT]
    Name                newrelic
    Match               *
    licenseKey          licenseKey
    proxy               https://https-proxy:3129
    ignoreSystemProxy   true
    caBundleFile        /cabundles/proxycert.pem
    caBundleDir         /cabundles
    validateProxyCerts  false
`

	outputBlock := FBCfgOutput{
		Name:       "newrelic",
		Match:      "*",
		LicenseKey: "licenseKey",
		// NOTE: the following proxy configuration is atypical, since we're providing both a bundle file and a bundle dir,
		// and we then force to skip the certificate validation. The purpose is to test that all fields are rendered
		// correctly in the resulting configuration file.
		Proxy:             "https://https-proxy:3129",
		IgnoreSystemProxy: true,
		CABundleFile:      "/cabundles/proxycert.pem",
		CABundleDir:       "/cabundles",
		ValidateCerts:     false,
	}

	fbCfg := FBCfg{
		Inputs: []FBCfgInput{
			{
				Name:          "tail",
				Tag:           "some-file",
				DB:            "fb.db",
				Path:          "file.foo",
				BufferMaxSize: "32k",
				SkipLongLines: "On",
			},
		},
		Parsers: []FBCfgParser{
			{
				Name:  "grep",
				Match: "some-file",
				Regex: "log foo",
			},
			{
				Name:  "record_modifier",
				Match: "*",
				Records: map[string]string{
					"entity.guid": "testGUID",
					"plugin.type": "nri-agent",
					"hostname":    "ubuntu",
				},
			},
		},
		Output: outputBlock,
	}

	result, cfgExt, err := fbCfg.Format()
	assert.Empty(t, err)
	assert.Empty(t, cfgExt)
	assert.Equal(t, expected, result)
}

func TestSyslogCorrectFormat(t *testing.T) {
	tests := []struct {
		name      string
		syslogCfg LogSyslogCfg
		ok        bool
	}{
		{
			"correct tcp/udp",
			LogSyslogCfg{
				URI: "tcp://0.0.0.0:1234",
			},
			true,
		},
		{
			"incorrect tcp",
			LogSyslogCfg{
				URI: "tcp:///0.0.0.0:1234",
			},
			false,
		},
		{
			"correct udp",
			LogSyslogCfg{
				URI: "udp://0.0.0.0:1234",
			},
			true,
		},
		{
			"incorrect udp 1",
			LogSyslogCfg{
				URI: "udp://0.0.0.0:",
			},
			false,
		},
		{
			"correct unix_udp",
			LogSyslogCfg{
				URI: "unix_udp:///var/test/socket",
			},
			true,
		},
		{
			"incorrect unix_udp",
			LogSyslogCfg{
				URI: "unix_udp://var/test/socket",
			},
			false,
		},
		{
			"correct unix_tcp",
			LogSyslogCfg{
				URI: "unix_tcp:///var/test/socket",
			},
			true,
		},
		{
			"unsupported protocol",
			LogSyslogCfg{
				URI: "invalid:///var/test/socket",
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newSyslogInput(tt.syslogCfg, "testTag", 32)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.NotEmpty(t, err)
			}
		})
	}
}

//go:build windows
// +build windows

/*
 * Copyright 2023 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package logs

import (
	"os"
	"strconv"
	"testing"

	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
)

//nolint:exhaustruct,dupl
func TestFBConfigForWinlog(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		logFwd   config.LogForward
		ohiCfg   LogsCfg
		expected FBCfg
	}{
		{
			"input winlog + eventId filtering + disable use ANSI", logFwdCfg,
			LogsCfg{
				{
					Name: "win-security",
					Winlog: &LogWinlogCfg{
						Channel:         "Security",
						CollectEventIds: []string{"5000", "6000-6100", "7000", "7900-8100"},
						ExcludeEventIds: []string{"6020-6060", "6070"},
						DisableUseANSI:  true,
					},
				},
			},

			FBCfg{
				Inputs: []FBCfgInput{
					{
						Name:     "winlog",
						Tag:      "win-security",
						DB:       dbDbPath,
						Channels: "Security",
					},
				},
				Filters: []FBCfgFilter{
					inputRecordModifier("winlog", "win-security"),
					{
						Name:   "lua",
						Match:  "win-security",
						Script: "Script.lua",
						Call:   "eventIdFilter",
					},
					{
						Name:  "modify",
						Match: "win-security",
						Modifiers: map[string]string{
							"Message":   "message",
							"EventType": "WinEventType",
						},
					},
					filterEntityBlock,
				},

				Output: outputBlock,
			},
		},
		{
			"input winlog + eventId filtering + use ANSI by Windows version", logFwdCfg,
			LogsCfg{
				{
					Name: "win-security",
					Winlog: &LogWinlogCfg{
						Channel:         "Security",
						CollectEventIds: []string{"5000", "6000-6100", "7000", "7900-8100"},
						ExcludeEventIds: []string{"6020-6060", "6070"},
					},
				},
			},

			FBCfg{
				Inputs: []FBCfgInput{
					{
						Name:     "winlog",
						Tag:      "win-security",
						DB:       dbDbPath,
						Channels: "Security",
					},
				},
				Filters: []FBCfgFilter{
					inputRecordModifier("winlog", "win-security"),
					{
						Name:   "lua",
						Match:  "win-security",
						Script: "Script.lua",
						Call:   "eventIdFilter",
					},
					{
						Name:  "modify",
						Match: "win-security",
						Modifiers: map[string]string{
							"Message":   "message",
							"EventType": "WinEventType",
						},
					},
					filterEntityBlock,
				},

				Output: outputBlock,
			},
		},
	}

	// "input winlog + eventId filtering + use ANSI by Windows version" test expectations
	// depend on Windows build number
	windowsBuildNumber := hostWindowsBuildNumber()
	if windowsBuildNumber != nil && *windowsBuildNumber <= winServer2016BuildNumber {
		tests[1].expected.Inputs[0].UseANSI = "True"
	}

	for _, testItem := range tests {
		// Prevent the loop variable from being captured in the closure below
		test := testItem

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fbConf, err := NewFBConf(test.ohiCfg, &logFwdCfg, "0", "")
			assert.NoError(t, err)
			assert.Equal(t, test.expected.Inputs, fbConf.Inputs)
			assert.Equal(t, test.expected.Filters[0], fbConf.Filters[0])
			assert.Equal(t, test.expected.Filters[1].Name, fbConf.Filters[1].Name)
			assert.Equal(t, test.expected.Filters[1].Match, fbConf.Filters[1].Match)
			assert.Equal(t, test.expected.Filters[1].Call, fbConf.Filters[1].Call)
			assert.Contains(t, fbConf.Filters[1].Script, "nr_fb_lua_filter")
			assert.Equal(t, test.expected.Filters[2], fbConf.Filters[2])
			assert.Equal(t, test.expected.Output, fbConf.Output)
			defer removeTempFile(t, fbConf.Filters[1].Script)
		})
	}
}

//nolint:exhaustruct,dupl
func TestFBConfigForWinevtlog(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		logFwd   config.LogForward
		ohiCfg   LogsCfg
		expected FBCfg
	}{
		{
			"input winevtlog + eventId filtering + disable use ANSI", logFwdCfg,
			LogsCfg{
				{
					Name: "win-security",
					Winevtlog: &LogWinevtlogCfg{
						Channel:         "Security",
						CollectEventIds: []string{"5000", "6000-6100", "7000", "7900-8100"},
						ExcludeEventIds: []string{"6020-6060", "6070"},
						DisableUseANSI:  true,
					},
				},
			},
			FBCfg{
				Inputs: []FBCfgInput{
					{
						Name:     "winevtlog",
						Tag:      "win-security",
						DB:       dbDbPath,
						Channels: "Security",
					},
				},
				Filters: []FBCfgFilter{
					inputRecordModifier("winevtlog", "win-security"),
					{
						Name:   "lua",
						Match:  "win-security",
						Script: "Script.lua",
						Call:   "eventIdFilter",
					},
					{
						Name:  "modify",
						Match: "win-security",
						Modifiers: map[string]string{
							"Message":   "message",
							"EventType": "WinEventType",
						},
					},
					filterEntityBlock,
				},

				Output: outputBlock,
			},
		},
		{
			"input winevtlog + eventId filtering + use ANSI by Windows version", logFwdCfg,
			LogsCfg{
				{
					Name: "win-security",
					Winevtlog: &LogWinevtlogCfg{
						Channel:         "Security",
						CollectEventIds: []string{"5000", "6000-6100", "7000", "7900-8100"},
						ExcludeEventIds: []string{"6020-6060", "6070"},
					},
				},
			},
			FBCfg{
				Inputs: []FBCfgInput{
					{
						Name:     "winevtlog",
						Tag:      "win-security",
						DB:       dbDbPath,
						Channels: "Security",
					},
				},
				Filters: []FBCfgFilter{
					inputRecordModifier("winevtlog", "win-security"),
					{
						Name:   "lua",
						Match:  "win-security",
						Script: "Script.lua",
						Call:   "eventIdFilter",
					},
					{
						Name:  "modify",
						Match: "win-security",
						Modifiers: map[string]string{
							"Message":   "message",
							"EventType": "WinEventType",
						},
					},
					filterEntityBlock,
				},

				Output: outputBlock,
			},
		},
	}

	// "input winevtlog + eventId filtering + use ANSI by Windows version" test expectations
	// depend on Windows build number
	windowsBuildNumber := hostWindowsBuildNumber()
	if windowsBuildNumber != nil && *windowsBuildNumber <= winServer2016BuildNumber {
		tests[1].expected.Inputs[0].UseANSI = "True"
	}

	for _, testItem := range tests {
		// Prevent the loop variable from being captured in the closure below
		test := testItem

		t.Run(testItem.name, func(t *testing.T) {
			t.Parallel()
			fbConf, err := NewFBConf(test.ohiCfg, &test.logFwd, "0", "")
			assert.NoError(t, err)
			assert.Equal(t, test.expected.Inputs, fbConf.Inputs)
			assert.Equal(t, test.expected.Filters[0], fbConf.Filters[0])
			assert.Equal(t, test.expected.Filters[1].Name, fbConf.Filters[1].Name)
			assert.Equal(t, test.expected.Filters[1].Match, fbConf.Filters[1].Match)
			assert.Equal(t, test.expected.Filters[1].Call, fbConf.Filters[1].Call)
			assert.Contains(t, fbConf.Filters[1].Script, "nr_fb_lua_filter")
			assert.Equal(t, test.expected.Filters[2], fbConf.Filters[2])
			assert.Equal(t, test.expected.Output, fbConf.Output)
			defer removeTempFile(t, fbConf.Filters[1].Script)
		})
	}
}

func hostWindowsBuildNumber() *int {
	hostInfo := getHostInfo()

	matches := platformBuildNumberRegex.FindStringSubmatch(hostInfo.PlatformVersion)

	if len(matches) == numMatchesExpected {
		if buildNumber, err := strconv.Atoi(matches[1]); err == nil {
			return &buildNumber
		}
	}

	return nil
}

func removeTempFile(t *testing.T, filePath string) {
	t.Helper()

	if err := os.Remove(filePath); err != nil {
		t.Log(err)
	}
}

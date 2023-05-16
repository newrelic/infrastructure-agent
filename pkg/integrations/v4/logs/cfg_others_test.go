//go:build !windows
// +build !windows

/*
 * Copyright 2023 New Relic Corporation. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package logs

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFBConfigForWinlog(t *testing.T) {
	nameTest := "input winlog + eventId filtering"
	input := LogsCfg{
		{
			Name: "win-security",
			Winlog: &LogWinlogCfg{
				Channel:         "Security",
				CollectEventIds: []string{"5000", "6000-6100", "7000", "7900-8100"},
				ExcludeEventIds: []string{"6020-6060", "6070"},
			},
		},
	}

	expected := FBCfg{
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
	}

	t.Run(nameTest, func(t *testing.T) {
		fbConf, err := NewFBConf(input, &logFwdCfg, "0", "")
		assert.NoError(t, err)
		assert.Equal(t, expected.Inputs, fbConf.Inputs)
		assert.Equal(t, expected.Filters[0], fbConf.Filters[0])
		assert.Equal(t, expected.Filters[1].Name, fbConf.Filters[1].Name)
		assert.Equal(t, expected.Filters[1].Match, fbConf.Filters[1].Match)
		assert.Equal(t, expected.Filters[1].Call, fbConf.Filters[1].Call)
		assert.Contains(t, fbConf.Filters[1].Script, "nr_fb_lua_filter")
		assert.Equal(t, expected.Filters[2], fbConf.Filters[2])
		assert.Equal(t, expected.Output, fbConf.Output)
		defer removeTempFile(t, fbConf.Filters[1].Script)
	})
}

func TestFBConfigForWinevtlog(t *testing.T) {
	nameTest := "input winevtlog + eventId filtering"
	input := LogsCfg{
		{
			Name: "win-security",
			Winevtlog: &LogWinevtlogCfg{
				Channel:         "Security",
				CollectEventIds: []string{"5000", "6000-6100", "7000", "7900-8100"},
				ExcludeEventIds: []string{"6020-6060", "6070"},
			},
		},
	}

	expected := FBCfg{
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
	}

	t.Run(nameTest, func(t *testing.T) {
		fbConf, err := NewFBConf(input, &logFwdCfg, "0", "")
		assert.NoError(t, err)
		assert.Equal(t, expected.Inputs, fbConf.Inputs)
		assert.Equal(t, expected.Filters[0], fbConf.Filters[0])
		assert.Equal(t, expected.Filters[1].Name, fbConf.Filters[1].Name)
		assert.Equal(t, expected.Filters[1].Match, fbConf.Filters[1].Match)
		assert.Equal(t, expected.Filters[1].Call, fbConf.Filters[1].Call)
		assert.Contains(t, fbConf.Filters[1].Script, "nr_fb_lua_filter")
		assert.Equal(t, expected.Filters[2], fbConf.Filters[2])
		assert.Equal(t, expected.Output, fbConf.Output)
		defer removeTempFile(t, fbConf.Filters[1].Script)
	})
}

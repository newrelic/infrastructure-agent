// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fflag

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/cmdchanneltest"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/service"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/os/api"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/assert"
)

var (
	l = log.WithComponent("test")
)

func Test_ffHandledState_requestWasAlreadyLogged(t *testing.T) {
	var s ffHandledState

	assert.Equal(t, ffNotHandledState, s)

	assert.False(t, s.requestWasAlreadyLogged(true))
	assert.Equal(t, ffHandledEnabledState, s)

	assert.True(t, s.requestWasAlreadyLogged(true))
	assert.Equal(t, ffHandledEnabledState, s)

	assert.False(t, s.requestWasAlreadyLogged(false))
	assert.Equal(t, ffHandledEnableAndDisableState, s)

	assert.True(t, s.requestWasAlreadyLogged(false))
	assert.Equal(t, ffHandledEnableAndDisableState, s)
}

func TestFFHandlerHandle_EnablesRegisterOnInitialFetch(t *testing.T) {
	c := config.Config{}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "register_enabled",
			"enabled": true }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

	assert.True(t, c.RegisterEnabled)
}

func TestFFHandlerHandle_DisablesRegisterOnInitialFetch(t *testing.T) {
	c := config.Config{RegisterEnabled: true}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "register_enabled",
			"enabled": false }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

	assert.False(t, c.RegisterEnabled)
}

func TestFFHandler_DMRegisterOnInitialFetch(t *testing.T) {
	tests := []struct {
		name         string
		feature      map[string]bool
		commandValue bool
		want         bool
	}{
		{
			name:         "Enabled if in config is enabled",
			feature:      map[string]bool{FlagDMRegisterEnable: true},
			commandValue: false,
			want:         true,
		},
		{
			name:         "Disabled if in config is disabled",
			feature:      map[string]bool{FlagDMRegisterEnable: false},
			commandValue: false,
			want:         false,
		},
		{
			name:         "Disabled if is not present in config and is disabled in command api",
			feature:      nil,
			commandValue: false,
			want:         false,
		},
		{
			name:         "Enabled if is not present in config and is enabled in command api",
			feature:      nil,
			commandValue: true,
			want:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := config.Config{Features: tc.feature}
			cmd := commandapi.Command{
				Args: []byte(fmt.Sprintf(`{
 			"category": "Infra_Agent",
			"flag": "dm_register_enabled",
			"enabled": %s }`, strconv.FormatBool(tc.commandValue))),
			}
			manager := feature_flags.NewManager(tc.feature)
			NewHandler(&config, manager, l).Handle(context.Background(), cmd, true)
			enable, _ := manager.GetFeatureFlag(FlagDMRegisterEnable)
			assert.Equal(t, tc.want, enable)
		})
	}
}

func TestFFHandlerHandle_DisablesParallelizeInventoryConfigOnInitialFetch(t *testing.T) {
	c := config.Config{
		InventoryQueueLen: 123,
	}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "parallelize_inventory_enabled",
			"enabled": false }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

	assert.Equal(t, 0, c.InventoryQueueLen)
}

func TestFFHandlerHandle_EnablesParallelizeInventoryConfigWithDefaultValue(t *testing.T) {
	c := config.Config{}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "parallelize_inventory_enabled",
			"enabled": true }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

	assert.Equal(t, CfgValueParallelizeInventory, int64(c.InventoryQueueLen))
}

func TestFFHandlerHandle_EnabledFFParallelizeInventoryDoesNotModifyProvidedConfig(t *testing.T) {
	c := config.Config{
		InventoryQueueLen: 123,
	}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "parallelize_inventory_enabled",
			"enabled": true }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

	assert.Equal(t, 123, c.InventoryQueueLen)
}

func TestFFHandlerHandle_ExitsOnDiffValueAndNotInitialFetch(t *testing.T) {
	type testCase struct {
		name string
		ff   string
	}
	testCases := []testCase{
		{
			name: "FF: register_enabled",
			ff:   FlagNameRegister,
		},
		{
			name: "FF: parallelize_inventory_enabled",
			ff:   FlagParallelizeInventory,
		},
	}
	for _, tc := range testCases {
		if os.Getenv("SHOULD_RUN_EXIT") == "1" {
			cmd := commandapi.Command{
				Args: []byte(fmt.Sprintf(`{
					"category": "Infra_Agent",
					"flag": "%s",
					"enabled": true }`, tc.ff)),
			}
			NewHandler(&config.Config{}, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, false)
		}

		cmd := exec.Command(os.Args[0], "-test.run=TestFFHandlerHandle_ExitsOnDiffValueAndNotInitialFetch")
		cmd.Env = append(os.Environ(), "SHOULD_RUN_EXIT=1")
		err := cmd.Run()
		if e, ok := err.(*exec.ExitError); ok && !e.Success() {
			if status, ok := e.Sys().(syscall.WaitStatus); ok {
				assert.Equal(t, api.ExitCodeRestart, status.ExitStatus())
				return
			}
			return
		}
		t.Fatalf("process ran with err %v, want exit status 1", err)
	}
}

func TestSrv_InitialFetch_EnablesRegister(t *testing.T) {
	serializedCmds := `
	{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "register_enabled",
					"enabled": true
				}
			}
		]
	}
`
	c := config.Config{RegisterEnabled: false}
	h := NewHandler(&c, feature_flags.NewManager(nil), l)
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", h.Handle)
	s := service.NewService(cmdchanneltest.SuccessClient(serializedCmds), 0, make(chan int, 1), ffHandler)

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	assert.True(t, c.RegisterEnabled)
}

func TestSrv_InitialFetch_DisablesRegister(t *testing.T) {
	serializedCmds := `
	{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "register_enabled",
					"enabled": false
				}
			}
		]
	}
`
	c := config.Config{RegisterEnabled: true}
	h := NewHandler(&c, feature_flags.NewManager(nil), l)
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", h.Handle)
	s := service.NewService(cmdchanneltest.SuccessClient(serializedCmds), 0, make(chan int, 1), ffHandler)

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	assert.False(t, c.RegisterEnabled)
}

func TestSrv_InitialFetch_EnablesDimensionalMetrics(t *testing.T) {
	serializedCmds := `
	{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "protocol_v4_enabled",
					"enabled": true
				}
			}
		]
	}
`
	ffManager := feature_flags.NewManager(nil)
	h := NewHandler(&config.Config{}, ffManager, l)
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", h.Handle)
	s := service.NewService(cmdchanneltest.SuccessClient(serializedCmds), 0, make(chan int, 1), ffHandler)

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	enabled, exists := ffManager.GetFeatureFlag(FlagProtocolV4)
	assert.True(t, exists)
	assert.True(t, enabled)
}

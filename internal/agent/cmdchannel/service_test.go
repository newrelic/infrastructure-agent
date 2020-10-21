// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package cmdchannel

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/os/api"
	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/log"

	"testing"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
)

var (
	l = log.WithComponent("test")
)

func TestFFHandlerHandle_EnablesRegisterOnInitialFetch(t *testing.T) {
	c := config.Config{}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "register_enabled",
			"enabled": true }`),
	}
	handler.NewFFHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

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
	handler.NewFFHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

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
			feature:      map[string]bool{handler.FlagDMRegisterEnable: true},
			commandValue: false,
			want:         true,
		},
		{
			name:         "Disabled if in config is disabled",
			feature:      map[string]bool{handler.FlagDMRegisterEnable: false},
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
			handler.NewFFHandler(&config, manager, l).Handle(context.Background(), cmd, true)
			enable, _ := manager.GetFeatureFlag(handler.FlagDMRegisterEnable)
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
	handler.NewFFHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

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
	handler.NewFFHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

	assert.Equal(t, handler.CfgValueParallelizeInventory, int64(c.InventoryQueueLen))
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
	handler.NewFFHandler(&c, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, true)

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
			ff:   handler.FlagNameRegister,
		},
		{
			name: "FF: parallelize_inventory_enabled",
			ff:   handler.FlagParallelizeInventory,
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
			handler.NewFFHandler(&config.Config{}, feature_flags.NewManager(nil), l).Handle(context.Background(), cmd, false)
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

func TestSrv_InitialFetch_ReturnsBackoff(t *testing.T) {
	serializedCmds := `
	{
		"return_value": [
			{
				"name": "backoff_command_channel",
				"arguments": {
					"delay": 3000
				}
			}
		]
	}
`
	s := NewService(cmdChannelClient(serializedCmds), &config.Config{}, feature_flags.NewManager(nil))

	initResp, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, time.Duration(3000)*time.Second, initResp.Delay)
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
	s := NewService(cmdChannelClient(serializedCmds), &c, feature_flags.NewManager(nil))

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
	s := NewService(cmdChannelClient(serializedCmds), &c, feature_flags.NewManager(nil))

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	assert.False(t, c.RegisterEnabled)
}

func Test_poll_DiscardsInvalidCommands(t *testing.T) {
	serializedCmds := `
	{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "THIS_FLAG_IS_NOT_MANAGED",
					"enabled": true
				}
			}
		]
	}
`
	s := NewService(cmdChannelClient(serializedCmds), &config.Config{}, feature_flags.NewManager(nil))

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)
}

func TestSrv_InitialFetch_EnablesRegisterAndHandlesBackoff(t *testing.T) {
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
			},
			{
				"id": 0,
				"name": "backoff_command_channel",
				"arguments": {
					"delay": 3000
				}
			}
		]
	}
`
	c := &config.Config{RegisterEnabled: false}
	ss := NewService(cmdChannelClient(serializedCmds), c, feature_flags.NewManager(nil))
	s := ss.(*srv)

	initialResp, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 3000*time.Second, initialResp.Delay)
	assert.Equal(t, 3000, s.pollDelaySecs)
	assert.True(t, c.RegisterEnabled)
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
	ss := NewService(cmdChannelClient(serializedCmds), &config.Config{RegisterEnabled: false}, ffManager)
	s := ss.(*srv)

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	enabled, exists := ffManager.GetFeatureFlag(handler.FlagProtocolV4)
	assert.True(t, exists)
	assert.True(t, enabled)
}

func TestSrv_Run(t *testing.T) {
	initialCmd := `
	{
		"return_value": [
			{
				"id": 0,
				"name": "backoff_command_channel",
				"arguments": {
					"delay": 0
				}
			}
		]
	}
`
	firstPollCmd := `
	{
		"return_value": [
			{
				"id": 0,
				"name": "backoff_command_channel",
				"arguments": {
					"delay": 2
				}
			},
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
	agentID := entity.ID(13)

	ffManager := feature_flags.NewManager(nil)
	cmdChClient, responsesCh, headerAgentIDCh := cmdChannelClientSpy(initialCmd, firstPollCmd)
	ss := NewService(cmdChClient, &config.Config{RegisterEnabled: false}, ffManager)
	s := ss.(*srv)

	type resp struct {
		icr InitialCmdResponse
		err error
	}
	var initialResp resp
	goRes := make(chan resp)
	go func() {
		icr, err := s.InitialFetch(context.Background())
		goRes <- resp{
			icr: icr,
			err: err,
		}
	}()

	<-responsesCh // discards response already returned in struct
	assert.Equal(t, entity.EmptyID, <-headerAgentIDCh)
	initialResp = <-goRes
	assert.NoError(t, initialResp.err)
	assert.Equal(t, time.Duration(0), initialResp.icr.Delay)
	assert.Equal(t, 0, s.pollDelaySecs, "before running minimum is not pre-set")

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		agentIdnProvideFn := func() entity.Identity {
			return entity.Identity{ID: agentID}
		}
		s.Run(ctx, agentIdnProvideFn, initialResp.icr)
		wg.Done()
	}()

	<-responsesCh // wait for response to be served
	assert.Equal(t, agentID, <-headerAgentIDCh)
	cancel()
	wg.Wait()
	assert.Equal(t, 2, s.pollDelaySecs, "minimum interval is 1sec")

	enabled, exists := ffManager.GetFeatureFlag(handler.FlagProtocolV4)
	assert.True(t, enabled)
	assert.True(t, exists)
}

func cmdChannelClient(serializedCmds string) commandapi.Client {
	httpClient := func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(serializedCmds))),
		}, nil
	}

	return commandapi.NewClient("https://foo", "123", "Agent v0", httpClient)
}

func cmdChannelClientSpy(serializedCmds ...string) (commandapi.Client, chan *http.Response, chan entity.ID) {
	requests := 0
	respCh := make(chan *http.Response)
	receivedAgentIDCh := make(chan entity.ID)

	httpClient := func(req *http.Request) (*http.Response, error) {
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(serializedCmds[requests]))),
		}
		requests++
		respCh <- resp
		agentID, _ := strconv.Atoi(req.Header.Get(http2.AgentEntityIdHeader))
		receivedAgentIDCh <- entity.ID(agentID)

		return resp, nil
	}

	return commandapi.NewClient("https://foo", "123", "Agent v0", httpClient), respCh, receivedAgentIDCh
}

func TestNewCmdHandler(t *testing.T) {
	type foo struct{ bar string }

	noopHandle := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (backoffSecs int, err error) {
		return
	}

	h := NewCmdHandler("foo", foo{}, noopHandle)

	assert.Equal(t, "foo", h.CmdName)
	assert.IsType(t, foo{}, h.CmdArgumentsType)

	// func comparison https://github.com/stretchr/testify/issues/182#issuecomment-495359313
	var (
		expectedF = runtime.FuncForPC(reflect.ValueOf(noopHandle).Pointer()).Name()
		gotF      = runtime.FuncForPC(reflect.ValueOf(h.Handle).Pointer()).Name()
	)
	assert.Equal(t, expectedF, gotF)
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package fflag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	log2 "github.com/newrelic/infrastructure-agent/test/log"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//nolint:gochecknoglobals
var (
	testLogger = log.WithComponent("test")
	errForTest = errors.New("some error")
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

	//nolint:errcheck
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, true)

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

	//nolint:errcheck
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, true)

	assert.False(t, c.RegisterEnabled)
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

	//nolint:errcheck
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, true)

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

	//nolint:errcheck
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, true)

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

	//nolint:errcheck
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, true)

	assert.Equal(t, 123, c.InventoryQueueLen)
}

func TestFFHandlerHandle_AsyncInventoryHandlerEnabledInitialFetch(t *testing.T) {
	c := config.Config{
		AsyncInventoryHandlerEnabled: false,
	}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "async_inventory_handler_enabled",
			"enabled": true }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, true)

	assert.True(t, c.AsyncInventoryHandlerEnabled)
}

func TestFFHandlerHandle_AsyncInventoryHandlerEnabled(t *testing.T) {
	c := config.Config{
		AsyncInventoryHandlerEnabled: true,
	}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "async_inventory_handler_enabled",
			"enabled": true }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, false)

	assert.True(t, c.AsyncInventoryHandlerEnabled)
}

func TestFFHandlerHandle_AsyncInventoryHandler_Disabled(t *testing.T) {
	c := config.Config{
		AsyncInventoryHandlerEnabled: true,
	}
	cmd := commandapi.Command{
		Args: []byte(`{
 			"category": "Infra_Agent",
			"flag": "async_inventory_handler_enabled",
			"enabled": false }`),
	}
	NewHandler(&c, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, true)

	assert.False(t, c.AsyncInventoryHandlerEnabled)
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

			//nolint:errcheck
			NewHandler(&config.Config{}, feature_flags.NewManager(nil), testLogger).Handle(context.Background(), cmd, false)
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
	h := NewHandler(&c, feature_flags.NewManager(nil), testLogger)
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
	h := NewHandler(&c, feature_flags.NewManager(nil), testLogger)
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
	h := NewHandler(&config.Config{}, ffManager, testLogger)
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", h.Handle)
	s := service.NewService(cmdchanneltest.SuccessClient(serializedCmds), 0, make(chan int, 1), ffHandler)

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	enabled, exists := ffManager.GetFeatureFlag(FlagProtocolV4)
	assert.True(t, exists)
	assert.True(t, enabled)
}

func TestSrv_InitialFetch_EnablesFb19(t *testing.T) {
	t.Parallel()

	serializedCmds := `
	{
		"return_value": [
			{
				"id": 0,
				"name": "set_feature_flag",
				"arguments": {
					"category": "Infra_Agent",
					"flag": "fluent_bit_19_win",
					"enabled": true
				}
			}
		]
	}
`
	ffManager := feature_flags.NewManager(nil)
	h := NewHandler(&config.Config{}, ffManager, testLogger)
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", h.Handle)
	s := service.NewService(cmdchanneltest.SuccessClient(serializedCmds), 0, make(chan int, 1), ffHandler)

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	enabled, exists := ffManager.GetFeatureFlag(FlagFluentBit19)
	assert.True(t, exists)
	assert.True(t, enabled)
}

//nolint:paralleltest
func Test_handleFBRestart_NoRestarter(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagSetterMock{}
	ffRetriever.ShouldReturnNoError(FlagFluentBit19)

	ffArgs := args{
		Category: FlagCategory,
		Flag:     FlagFluentBit19,
		Enabled:  false,
	}

	hook := log2.NewInMemoryEntriesHook([]logrus.Level{logrus.DebugLevel})
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	h := NewHandler(&config.Config{}, ffRetriever, testLogger)
	h.handleFBRestart(ffArgs)

	entries := hook.GetEntries()
	assert.Equal(t, "No fbRestarter for cmd feature request.", entries[0].Message)

	mock.AssertExpectationsForObjects(t, ffRetriever)
}

//nolint:paralleltest
func Test_handleFBRestart_WithRestarterNoError(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagSetterMock{}
	ffRetriever.ShouldReturnNoError(FlagFluentBit19)

	ffArgs := args{
		Category: FlagCategory,
		Flag:     FlagFluentBit19,
		Enabled:  false,
	}

	hook := log2.NewInMemoryEntriesHook([]logrus.Level{logrus.DebugLevel})
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	restarter := &FBRestarterMock{}
	restarter.ShouldReturnNoError()

	h := NewHandler(&config.Config{}, ffRetriever, testLogger)
	h.SetFBRestarter(restarter)
	h.handleFBRestart(ffArgs)

	entries := hook.GetEntries()
	assert.Len(t, entries, 0)

	mock.AssertExpectationsForObjects(t, ffRetriever, restarter)
}

//nolint:paralleltest
func Test_handleFBRestart_WithRestarterError(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagSetterMock{}
	ffRetriever.ShouldReturnNoError(FlagFluentBit19)

	ffArgs := args{
		Category: FlagCategory,
		Flag:     FlagFluentBit19,
		Enabled:  false,
	}

	hook := log2.NewInMemoryEntriesHook([]logrus.Level{logrus.DebugLevel})
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	restarter := &FBRestarterMock{}
	restarter.ShouldReturnError(errForTest)

	h := NewHandler(&config.Config{}, ffRetriever, testLogger)
	h.SetFBRestarter(restarter)
	h.handleFBRestart(ffArgs)

	entries := hook.GetEntries()
	assert.Equal(t, "Unable to restart fb", entries[0].Message)
	assert.ErrorIs(t, errForTest, entries[0].Data["error"].(error)) //nolint:forcetypeassert

	mock.AssertExpectationsForObjects(t, ffRetriever, restarter)
}

//nolint:paralleltest
func Test_handleFBRestart_FlagAlreadyExists(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagSetterMock{}
	ffRetriever.ShouldReturnError(FlagFluentBit19, feature_flags.ErrFeatureFlagAlreadyExists)

	ffArgs := args{
		Category: FlagCategory,
		Flag:     FlagFluentBit19,
		Enabled:  false,
	}

	hook := log2.NewInMemoryEntriesHook([]logrus.Level{logrus.DebugLevel})
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	restarter := &FBRestarterMock{}

	h := NewHandler(&config.Config{}, ffRetriever, testLogger)
	h.SetFBRestarter(restarter)
	h.handleFBRestart(ffArgs)

	entries := hook.GetEntries()
	assert.Len(t, entries, 0)

	mock.AssertExpectationsForObjects(t, ffRetriever, restarter)
}

//nolint:paralleltest
func Test_handleFBRestart_FlagRetrieverError(t *testing.T) {
	ffRetriever := &feature_flags.FeatureFlagSetterMock{}
	ffRetriever.ShouldReturnError(FlagFluentBit19, errForTest)

	ffArgs := args{
		Category: FlagCategory,
		Flag:     FlagFluentBit19,
		Enabled:  false,
	}

	hook := log2.NewInMemoryEntriesHook([]logrus.Level{logrus.DebugLevel})
	log.AddHook(hook)
	log.SetLevel(logrus.DebugLevel)

	restarter := &FBRestarterMock{}

	h := NewHandler(&config.Config{}, ffRetriever, testLogger)
	h.SetFBRestarter(restarter)
	h.handleFBRestart(ffArgs)

	entries := hook.GetEntries()
	assert.Len(t, entries, 1)
	assert.Equal(t, "Cannot set feature flag configuration.", entries[0].Message)
	assert.ErrorIs(t, errForTest, entries[0].Data["error"].(error)) //nolint:forcetypeassert

	mock.AssertExpectationsForObjects(t, ffRetriever, restarter)
}

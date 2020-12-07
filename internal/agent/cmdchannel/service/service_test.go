// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/backoff"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/cmdchanneltest"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/fflag"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/runintegration"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	http2 "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	dm "github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm/testutils"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/stretchr/testify/require"

	"testing"

	"github.com/newrelic/infrastructure-agent/internal/feature_flags"

	"github.com/newrelic/infrastructure-agent/pkg/backend/commandapi"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/stretchr/testify/assert"
)

var (
	l = log.WithComponent("test")
)

func TestNewCmdHandler(t *testing.T) {
	noopHandle := func(ctx context.Context, cmd commandapi.Command, initialFetch bool) (err error) {
		return
	}

	h := cmdchannel.NewCmdHandler("foo", noopHandle)

	assert.Equal(t, "foo", h.CmdName)
	// func comparison https://github.com/stretchr/testify/issues/182#issuecomment-495359313
	var (
		expectedF = runtime.FuncForPC(reflect.ValueOf(noopHandle).Pointer()).Name()
		gotF      = runtime.FuncForPC(reflect.ValueOf(h.Handle).Pointer()).Name()
	)
	assert.Equal(t, expectedF, gotF)
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
	s := NewService(cmdchanneltest.SuccessClient(serializedCmds), 0, make(chan int, 1))

	_, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)
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
	boC := make(chan int, 1)
	s := NewService(cmdchanneltest.SuccessClient(serializedCmds), 1, boC, backoff.NewHandler(boC))

	initResp, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, time.Duration(3000)*time.Second, initResp.Delay)
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
	h := fflag.NewHandler(c, feature_flags.NewManager(nil), l)
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", h.Handle)
	boC := make(chan int, 1)
	ss := NewService(cmdchanneltest.SuccessClient(serializedCmds), 0, boC, backoff.NewHandler(boC), ffHandler)
	s := ss.(*srv)

	initialResp, err := s.InitialFetch(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, 3000*time.Second, initialResp.Delay)
	assert.Equal(t, 3000, s.pollDelaySecs)
	assert.True(t, c.RegisterEnabled)
}

func TestSrv_InitialFetch_HandlesRunIntegrationAndMetadata(t *testing.T) {
	serializedCmds := `
	{
		"return_value": [
			{
				"name": "run_integration",
				"arguments": {
					"integration_name": "nri-foo"
				},
				"metadata": {
					"target_pid": 123,
					"target_strategy": "<STRATEGY>"
				}
			}
		]
	}
`
	defQueue := make(chan integration.Definition, 1)
	il := integration.InstancesLookup{
		ByName: func(_ string) (string, error) {
			return "/path/to/nri-foo", nil
		},
	}
	h := runintegration.NewHandler(defQueue, il, dm.NewNoopEmitter(), l)

	s := NewService(cmdchanneltest.SuccessClient(serializedCmds), 1, make(chan int, 1), h)

	_, err := s.InitialFetch(context.Background())
	require.NoError(t, err)

	d := <-defQueue
	assert.Equal(t, "nri-foo", d.Name)
	require.NotNil(t, d.CmdChanReq)
	require.Contains(t, d.CmdChanReq.Metadata, "target_pid")
	require.Contains(t, d.CmdChanReq.Metadata, "target_strategy")
	assert.Equal(t, float64(123), d.CmdChanReq.Metadata["target_pid"])
	assert.Equal(t, "<STRATEGY>", d.CmdChanReq.Metadata["target_strategy"])
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
	c := &config.Config{RegisterEnabled: false}
	h := fflag.NewHandler(c, ffManager, l)
	ffHandler := cmdchannel.NewCmdHandler("set_feature_flag", h.Handle)

	cmdChClient, responsesCh, headerAgentIDCh := cmdChannelClientIDSpy(initialCmd, firstPollCmd)
	boC := make(chan int, 1)
	ss := NewService(cmdChClient, 0, boC, backoff.NewHandler(boC), ffHandler)
	s := ss.(*srv)

	type resp struct {
		icr cmdchannel.InitialCmdResponse
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
	assert.Equal(t, 2, <-s.pollDelaySecsC, "minimum interval is 1sec")
	cancel()
	wg.Wait()

	enabled, exists := ffManager.GetFeatureFlag(fflag.FlagProtocolV4)
	assert.True(t, enabled)
	assert.True(t, exists)
}

func cmdChannelClientIDSpy(serializedCmds ...string) (commandapi.Client, chan *http.Response, chan entity.ID) {
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

func TestSrv_Run_HandlesRunIntegrationAndACKs(t *testing.T) {
	defQueue := make(chan integration.Definition, 1)
	il := integration.InstancesLookup{
		ByName: func(_ string) (string, error) {
			return "/path/to/nri-foo", nil
		},
	}
	h := runintegration.NewHandler(defQueue, il, dm.NewNoopEmitter(), l)

	cmd := `
	{
		"return_value": [
			{
				"id":   0,
				"hash": "xyz",
				"name": "run_integration",
				"arguments": {
					"integration_name": "nri-foo"
				}
			}
		]
	}`
	cmdChClient, requestsCh := ccClientRequestsSpyReturning(cmd)
	s := NewService(cmdChClient, 0, make(chan int, 1), h)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		agentIdnProvideFn := func() entity.Identity {
			return entity.Identity{ID: 123}
		}
		s.Run(ctx, agentIdnProvideFn, cmdchannel.InitialCmdResponse{})
	}()

	req1 := <-requestsCh
	req2 := <-requestsCh
	cancel()

	assert.Equal(t, http.MethodGet, req1.Method, "GET commands request is expected")
	assert.Equal(t, http.MethodPost, req2.Method, "POST ack submission is expected")

	d := <-defQueue
	assert.Equal(t, "nri-foo", d.Name)
}

func ccClientRequestsSpyReturning(payload string) (commandapi.Client, <-chan *http.Request) {
	reqs := make(chan *http.Request)
	httpClient := func(req *http.Request) (*http.Response, error) {
		reqs <- req
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(payload))),
		}, nil
	}

	return commandapi.NewClient("https://foo", "123", "Agent v0", httpClient), reqs
}

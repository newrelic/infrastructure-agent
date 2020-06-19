// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package commandapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

// Available commands
const (
	setFFCmd   = "set_feature_flag"
	backoffCmd = "backoff_command_channel"
)

var (
	// Errors
	UnknownCmdErr = errors.New("unknown command name")
)

type Client interface {
	GetCommands(agentID entity.ID) ([]Command, error)
}

type Command struct {
	ID   int         `json:"id"`
	Name string      `json:"name"`
	Args interface{} `json:"arguments"`
}

func (c *Command) UnmarshalJSON(b []byte) (err error) {
	var rawArgs json.RawMessage
	c.Args = &rawArgs

	type cc Command // avoid infinite nesting
	if err = json.Unmarshal(b, (*cc)(c)); err != nil {
		return err
	}

	switch c.Name {
	case backoffCmd:
		var boArgs BackoffArgs
		if err = json.Unmarshal(rawArgs, &boArgs); err != nil {
			return
		}
		c.Args = boArgs
	case setFFCmd:
		var ffArgs FFArgs
		if err = json.Unmarshal(rawArgs, &ffArgs); err != nil {
			return
		}
		c.Args = ffArgs
	default:
		// returning partial cmd as it might bundle useful info
		err = UnknownCmdErr
	}

	return
}

type BackoffArgs struct {
	Delay int // backoff delay in secs
}

type FFArgs struct {
	Category string
	Flag     string
	Enabled  bool
}

func (f *FFArgs) Apply(cfg *config.Config) {}

type client struct {
	svcURL     string
	licenseKey string
	userAgent  string
	httpClient backendhttp.Client
}

func (c *client) GetCommands(agentID entity.ID) ([]Command, error) {
	req, err := http.NewRequest("GET", c.svcURL, nil)
	if err != nil {
		return nil, fmt.Errorf("command request creation failed: %s", err)
	}

	resp, err := c.do(req, agentID)
	if err != nil {
		return nil, fmt.Errorf("command request submission failed: %s", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read server response: %s", err)
	}

	if backendhttp.IsResponseError(resp) {
		return nil, fmt.Errorf("unsuccessful response, status:%d [%s]", resp.StatusCode, string(body))
	}

	return unmarshalCmdChannelPayload(body)
}

func NewClient(svcURL, licenseKey, userAgent string, httpClient backendhttp.Client) Client {
	return &client{
		svcURL:     strings.TrimSuffix(svcURL, "/"),
		licenseKey: licenseKey,
		userAgent:  userAgent,
		httpClient: httpClient,
	}
}

func (c *client) do(req *http.Request, agentID entity.ID) (*http.Response, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set(backendhttp.LicenseHeader, c.licenseKey)
	req.Header.Set(backendhttp.AgentEntityIdHeader, agentID.String()) // ok being 0, ie: at startup

	return c.httpClient(req)
}

func unmarshalCmdChannelPayload(payload []byte) (cmds []Command, err error) {
	var f map[string][]Command
	if err := json.Unmarshal(payload, &f); err != nil {
		return nil, fmt.Errorf("unable to decode server cmd payload: %s", string(payload))
	}

	for _, c := range f["return_value"] {
		cmds = append(cmds, c)
	}

	return
}

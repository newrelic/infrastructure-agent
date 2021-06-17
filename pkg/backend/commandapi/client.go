// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package commandapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	backendhttp "github.com/newrelic/infrastructure-agent/pkg/backend/http"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
)

type Client interface {
	GetCommands(agentID entity.ID) ([]Command, error)
	AckCommand(agentID entity.ID, cmdHash string) error
}

type Command struct {
	ID       int                    `json:"id"`
	Hash     string                 `json:"hash"`
	Metadata map[string]interface{} `json:"metadata"`
	Name     string                 `json:"name"`
	Args     json.RawMessage        `json:"arguments"`
}

type client struct {
	svcURL     string
	licenseKey string
	userAgent  string
	httpClient backendhttp.Client
}

func NewClient(svcURL, licenseKey, userAgent string, httpClient backendhttp.Client) Client {
	return &client{
		svcURL:     strings.TrimSuffix(svcURL, "/"),
		licenseKey: licenseKey,
		userAgent:  userAgent,
		httpClient: httpClient,
	}
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

func (c *client) AckCommand(agentID entity.ID, cmdHash string) error {
	payload := strings.NewReader(fmt.Sprintf(`{ "hash": "%s", "name": "ack" }`, cmdHash))

	req, err := http.NewRequest("POST", c.svcURL, payload)
	if err != nil {
		return fmt.Errorf("cmd channel ack request creation failed: %s", err)
	}

	resp, err := c.do(req, agentID)
	if err != nil {
		return fmt.Errorf("cmd channel ack request submission failed: %s", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if !backendhttp.IsResponseError(resp) {
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		body = []byte(fmt.Sprintf("cannot read cmd channel ack response: %s", err.Error()))
	}

	return fmt.Errorf("unsuccessful ack, status:%d [%s]", resp.StatusCode, string(body))
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

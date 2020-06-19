// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
// +build linux darwin

package linux

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/infrastructure-agent/pkg/log"

	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"

	"github.com/kolo/xmlrpc"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/helpers"
)

var slog = log.WithPlugin("Supervisor")

type Supervisor interface {
	Processes() ([]SupervisorProcess, error)
}

type SupervisorPlugin struct {
	agent.PluginCommon
	proto, addr string
	supervisor  Supervisor
	frequency   time.Duration
}

type SupervisorItem struct {
	Name string `json:"id"`
	Pid  string `json:"pid"`
}

func (self SupervisorItem) SortKey() string {
	return self.Name
}

func processSupervisorRpcSocket(socketAddr string) (string, string) {
	if strings.HasPrefix(socketAddr, "/") {
		return "unix", socketAddr
	} else {
		return "http", socketAddr
	}
}

func NewSupervisorPlugin(id ids.PluginID, ctx agent.AgentContext) agent.Plugin {
	proto, addr := processSupervisorRpcSocket(ctx.Config().SupervisorRpcSocket)
	cfg := ctx.Config()
	return &SupervisorPlugin{
		PluginCommon: agent.PluginCommon{ID: id, Context: ctx},
		proto:        proto,
		addr:         addr,
		frequency: config.ValidateConfigFrequencySetting(
			cfg.SupervisorRefreshSec,
			config.FREQ_MINIMUM_FAST_INVENTORY_SAMPLE_RATE,
			config.FREQ_PLUGIN_SUPERVISOR_UPDATES,
			cfg.DisableAllPlugins,
		) * time.Second,
	}
}

func (self *SupervisorPlugin) GetClient() (cl Supervisor, err error) {
	if self.supervisor != nil {
		return self.supervisor, nil
	}
	cl, err = NewSupervisorClient(self.proto, self.addr)
	if err != nil {
		return
	}
	self.supervisor = cl
	return
}

func (self *SupervisorPlugin) CanRun() bool {
	client, err := self.GetClient()
	if err != nil {
		slog.WithError(err).Debug("Connecting to supervisord, not running or misconfigured.")
		return false
	}
	_, err = client.Processes()
	if err != nil {
		slog.WithError(err).Error("Communicating with supevisord, not running or misconfigured")
	}
	return err == nil
}

func (self *SupervisorPlugin) Data() (agent.PluginInventoryDataset, map[int]string, error) {
	client, err := self.GetClient()
	if err != nil {
		return nil, nil, err
	}
	procs, err := client.Processes()
	if err != nil {
		return nil, nil, err
	}
	a := agent.PluginInventoryDataset{}
	pidMap := make(map[int]string)
	for _, proc := range procs {
		if proc.State != 10 && proc.State != 20 {
			continue
		}
		v := SupervisorItem{
			proc.Name,
			strconv.FormatInt(proc.Pid, 10),
		}
		pidMap[int(proc.Pid)] = proc.Name
		a = append(a, v)
	}
	return a, pidMap, nil
}

func (self *SupervisorPlugin) Run() {
	if self.frequency <= config.FREQ_DISABLE_SAMPLING {
		slog.Debug("Disabled.")
		return
	}

	if !self.CanRun() {
		self.Unregister()
		return
	}
	firstTime := true
	for {
		if firstTime {
			firstTime = false
		} else {
			time.Sleep(self.frequency)
		}
		data, pidMap, err := self.Data()
		if err != nil {
			slog.WithError(err).Error("getting supervisord data")
			continue
		}
		self.EmitInventory(data, self.Context.AgentIdentifier())
		self.Context.CacheServicePids(sysinfo.PROCESS_NAME_SOURCE_SUPERVISOR, pidMap)
	}
}

type SupervisorClient struct {
	client *xmlrpc.Client
}

type SupervisorProcess struct {
	Name          string `xmlrpc:"name"`
	Group         string `xmlrpc:"group"`
	Start         int64  `xmlrpc:"start"`
	Stop          int64  `xmlrpc:"stop"`
	Now           int64  `xmlrpc:"now"`
	State         int    `xmlrpc:"state"`
	StateName     string `xmlrpc:"statename"`
	SpawnError    string `xmlrpc:"spawnerr"`
	ExitStatus    int    `xmlrpc:"exitstatus"`
	StdOutLogFile string `xmlrpc:"stdout_logfile"`
	StdErrLogFile string `xmlrpc:"stderr_logfile"`
	Pid           int64  `xmlrpc:"pid"`
}

func NewSupervisorClient(proto string, address string) (cl *SupervisorClient, err error) {
	var xmlCl *xmlrpc.Client
	switch proto {
	case "unix":
		var sockTransport *helpers.PersistentSocketTransport
		if sockTransport, err = helpers.NewSocketTransport(address); err != nil {
			return
		}
		transport := &http.Transport{}
		transport.RegisterProtocol("unix", sockTransport)
		xmlCl, err = xmlrpc.NewClient("unix:///RPC2", transport)
	case "http":
		xmlCl, err = xmlrpc.NewClient(address, nil)
	default:
		err = fmt.Errorf("supervisord client: unknown protocol '%s'", proto)
	}
	if err != nil {
		return
	}
	cl = &SupervisorClient{xmlCl}
	return
}

func (self *SupervisorClient) Processes() ([]SupervisorProcess, error) {
	procs := []SupervisorProcess{}
	err := self.client.Call("supervisor.getAllProcessInfo", nil, &procs)
	if err != nil {
		return nil, fmt.Errorf("supervisor.Processes: %v", err)
	}
	return procs, nil
}

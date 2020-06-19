// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package proxy

import (
	"net/url"
	"os"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
)

const (
	typeFile = "file"
	typeDir  = "directory"
)

var proxyConfigID = ids.PluginID{Category: "metadata", Term: "proxy_config"}

// entry represents any entry from the proxy information
type entry struct {
	Id string `json:"id"`
}

// proxyEntry shows the scheme of the proxy URL or an error if there was a problem parsing it
type proxyEntry struct {
	entry
	Scheme string `json:"scheme,omitempty"`
	Error  string `json:"error,omitempty"`
}

// fileEntry shows the type of file belonging to a path, or an error if there was a problem reading it
// we are not considering errors here because the agent won't start if the CA file/directory is not found
type fileEntry struct {
	entry
	Type string `json:"type,omitempty"` // file or dir
}

// boolEntry represents a true/false proxy configuration entry
type boolEntry struct {
	entry
	Value bool `json:"value"`
}

func (e *entry) SortKey() string {
	return e.Id
}

// ProxyConfigPlugins reports the ProxyConfig as inventory
type configPlugin struct {
	agent.PluginCommon
	config []agent.Sortable
}

func ConfigPlugin(ctx agent.AgentContext) agent.Plugin {
	cfg := ctx.Config()
	if cfg == nil {
		cfg = &config.Config{} // empty config to avoid null pointers
	}

	proxyConfig := make([]agent.Sortable, 0)

	if e := urlEntry(os.Getenv("HTTPS_PROXY")); e != nil {
		e.Id = "HTTPS_PROXY"
		proxyConfig = append(proxyConfig, e)
	}
	if e := urlEntry(os.Getenv("HTTP_PROXY")); e != nil {
		e.Id = "HTTP_PROXY"
		proxyConfig = append(proxyConfig, e)
	}
	if e := urlEntry(cfg.Proxy); e != nil {
		e.Id = "proxy"
		proxyConfig = append(proxyConfig, e)
	}
	if e := pathEntry(cfg.CABundleDir); e != nil {
		e.Id = "ca_bundle_dir"
		proxyConfig = append(proxyConfig, e)
	}
	if e := pathEntry(cfg.CABundleFile); e != nil {
		e.Id = "ca_bundle_file"
		proxyConfig = append(proxyConfig, e)
	}
	proxyConfig = append(proxyConfig, &boolEntry{
		entry: entry{Id: "ignore_system_proxy"},
		Value: cfg.IgnoreSystemProxy,
	})
	proxyConfig = append(proxyConfig, &boolEntry{
		entry: entry{Id: "proxy_validate_certificates"},
		Value: cfg.ProxyValidateCerts,
	})

	return &configPlugin{
		agent.PluginCommon{ID: proxyConfigID, Context: ctx},
		proxyConfig,
	}
}

func urlEntry(rawUrl string) *proxyEntry {
	if rawUrl == "" {
		return nil
	}
	u, err := url.Parse(rawUrl)
	if err != nil {
		// putting generic error message to avoid filtering the proxy url, by security reasons
		return &proxyEntry{
			Scheme: "",
			Error:  "wrong url",
		}
	}
	return &proxyEntry{
		Scheme: u.Scheme,
		Error:  "",
	}
}

func pathEntry(path string) *fileEntry {
	if path == "" {
		return nil
	}
	stat, err := os.Stat(path)
	if err != nil {
		// this is not likely to happen, but sending some information instead of nil
		// putting generic error message to avoid filtering the CA path, by security reasons
		return &fileEntry{
			Type: "unexpected error",
		}
	}
	if stat.IsDir() {
		return &fileEntry{Type: typeDir}
	}
	return &fileEntry{Type: typeFile}
}

func (pcp *configPlugin) Run() {
	pcp.Context.AddReconnecting(pcp)

	pcp.EmitInventory(pcp.config, pcp.Context.AgentIdentifier())
}

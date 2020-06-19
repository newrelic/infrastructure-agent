// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/newrelic/infrastructure-agent/pkg/integrations/legacy"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"
)

type HTTPServerPlugin struct {
	agent.PluginCommon
	host   string
	port   int
	logger log.Entry
}

type responseError struct {
	Error string `json:"error"`
}

func NewHTTPServerPlugin(ctx agent.AgentContext, host string, port int) agent.Plugin {
	id := ids.PluginID{
		Category: "metadata",
		Term:     "http_server",
	}
	return &HTTPServerPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      id,
			Context: ctx,
		},
		host:   host,
		port:   port,
		logger: slog.WithPlugin(id.String()),
	}
}

func (p *HTTPServerPlugin) Run() {
	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	router := httprouter.New()
	router.POST("/v1/data", p.dataHandler)
	p.logger.WithFields(logrus.Fields{
		"port": p.port,
		"host": p.host,
	}).Debug("HTTP server starting listening.")
	err := http.ListenAndServe(addr, router)
	if err != nil {
		p.logger.WithError(err).Error("unable to start HTTP server")
	}
}

func (p *HTTPServerPlugin) dataHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	rawBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		p.logger.WithError(err).Debug("Reading request body.")
	}
	payload, protocolVersion, err := legacy.ParsePayload(rawBody, p.Context.Config().ForceProtocolV2toV3)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		jerr := json.NewEncoder(w).Encode(responseError{
			Error: fmt.Sprintf("error decoding data payload: %v", err),
		})
		if jerr != nil {
			p.logger.WithError(err).Warn("couldn't encode a failed response")
		}
		return
	}
	labels := map[string]string{}
	for _, dataSet := range payload.DataSets {
		err := legacy.EmitDataSet(
			p.Context,
			p,
			payload.Name,
			payload.IntegrationVersion,
			"",
			dataSet,
			labels,
			labels,
			nil,
			protocolVersion)
		if err != nil {
			p.logger.WithError(err).Warn("emitting plugin dataset")
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/emitter"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/sirupsen/logrus"
)

const IntegrationName = "http-api"

type HTTPServerPlugin struct {
	agent.PluginCommon
	host       string
	port       int
	logger     log.Entry
	definition integration.Definition
	emitter    emitter.Emitter
}

type responseError struct {
	Error string `json:"error"`
}

func NewHTTPServerPlugin(ctx agent.AgentContext, host string, port int, em emitter.Emitter) (p agent.Plugin, err error) {
	id := ids.PluginID{
		Category: "metadata",
		Term:     "http_server",
	}

	logger := slog.WithPlugin(id.String())

	d, err := integration.NewAPIDefinition(IntegrationName)
	if err != nil {
		err = fmt.Errorf("cannot create API definition for HTTP API server, err: %s", err)
		return
	}

	p = &HTTPServerPlugin{
		PluginCommon: agent.PluginCommon{
			ID:      id,
			Context: ctx,
		},
		host:       host,
		port:       port,
		logger:     logger,
		definition: d,
		emitter:    em,
	}
	return
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
		errMsg := "cannot read HTTP payload"
		p.logger.WithError(err).Warn(errMsg)
		w.WriteHeader(http.StatusBadRequest)
		jerr := json.NewEncoder(w).Encode(responseError{
			Error: fmt.Sprintf("%s: %s", errMsg, err.Error()),
		})
		if jerr != nil {
			p.logger.WithError(jerr).Warn("couldn't encode a failed response")
		}
		return
	}

	err = p.emitter.Emit(p.definition, nil, nil, rawBody)
	if err != nil {
		errMsg := "cannot emit HTTP payload"
		p.logger.WithError(err).Warn(errMsg)
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(fmt.Sprintf("%s, err: %s", errMsg, err.Error())))
		if err != nil {
			p.logger.WithError(err).Warn("cannot write HTTP response body")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

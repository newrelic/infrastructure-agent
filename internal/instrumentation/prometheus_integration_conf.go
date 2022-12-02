// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instrumentation

import (
	"context"
	"os"
	"text/template"
)

var promIntConf = `
# TYPE newrelic_infra_instrumentation_dm_requests_forwarded counter
integrations:
  - name: nri-prometheus
    config:
      # When standalone is set to false nri-prometheus requires an infrastructure agent to work and send data. Defaults to true
      standalone: false

      # When running with infrastructure agent emitters will have to include infra-sdk
      emitters: infra-sdk

      targets:
        - description: Agent internal exporter
          urls: ["{{.AgentMetricsEndpoint}}"]

      # Whether the integration should run in verbose mode or not. Defaults to false.
      verbose: true

      # Length in time to distribute the scraping from the endpoints.
      scrape_duration: "5s"

      # Whether the integration should skip TLS verification or not. Defaults to false.
      insecure_skip_verify: true

    timeout: 10s
`

// TODO set correct path somehow for windows _windows?
var promIntConfPath = "/etc/newrelic-infra/integrations.d/nr-agent-prometheus-config.yml"

func SetupPrometheusIntegrationConfig(ctx context.Context, agentMetricsEndpoint string) error {
	err := createConfigFile(agentMetricsEndpoint)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		cleanConfigFile()
	}()

	return nil
}

func cleanConfigFile() {
	err := os.Remove(promIntConfPath)
	if err != nil {
		//TODO wlog.WithError(err) import cycle?
	}
}

func createConfigFile(agentMetricsEndpoint string) error {
	cnf := struct {
		AgentMetricsEndpoint string
	}{
		AgentMetricsEndpoint: agentMetricsEndpoint,
	}

	tmpl, err := template.New("promIntConf").Parse(promIntConf)
	if err != nil {
		return err
	}

	f, err := os.Create(promIntConfPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, cnf)
}

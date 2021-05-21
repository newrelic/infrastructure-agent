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

      # The name of your cluster. It's important to match other New Relic products to relate the data.
      #cluster_name: "my_exporter"

      targets:
        - description: Agent internal exporter
          urls: ["{{.AgentMetricsEndpoint}}"]


      # Whether the integration should run in verbose mode or not. Defaults to false.
      verbose: true

      # Whether the integration should run in audit mode or not. Defaults to false.
      # Audit mode logs the uncompressed data sent to New Relic. Use this to log all data sent.
      # It does not include verbose mode. This can lead to a high log volume, use with care.
      audit: false

      # The HTTP client timeout when fetching data from endpoints. Defaults to 30s.
      # scrape_timeout: "30s"

      # Length in time to distribute the scraping from the endpoints.
      scrape_duration: "5s"

      # Number of worker threads used for scraping targets.
      # Increasing this value too much will result in huge memory consumption if too
      # many metrics are being scraped.
      # Default: 4
      # worker_threads: 4

      # Whether the integration should skip TLS verification or not. Defaults to false.
      insecure_skip_verify: true

    timeout: 10s
`

var promIntConfPath = "/etc/newrelic-infra/integrations.d/nr-agent-prometheus-config.yml"

func SetupPrometheusIntegrationConfig(ctx context.Context, agentMetricsEndpoint string) error {
	err := createConfigFile(agentMetricsEndpoint)
	if err != nil {
		return err
	}

	go func(){
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

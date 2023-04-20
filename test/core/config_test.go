package core

import (
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/cloud"
	"github.com/newrelic/infrastructure-agent/test/infra"
	ihttp "github.com/newrelic/infrastructure-agent/test/infra/http"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// TestHttpHeaders asserts that http requests performed by the agent contains the http headers specified in the configuration.
func TestHttpHeaders_Inventory(t *testing.T) {
	const timeout = 5 * time.Second

	testClient := ihttp.NewRequestRecorderClient()
	a := infra.NewAgent(testClient.Client, func(config *config.Config) {
		config.HttpHeaders["test_key"] = "test_value"
	})

	a.Context.SetAgentIdentity(entity.Identity{
		ID:   10,
		GUID: "abcdef",
	})

	cloudDetector := cloud.NewDetector(true, 0, 0, 0, false)
	a.RegisterPlugin(plugins.NewHostAliasesPlugin(a.Context, cloudDetector))
	a.RegisterPlugin(plugins.NewAgentConfigPlugin(*ids.NewPluginID("metadata", "agent_config"), a.Context))

	go a.Run()

	select {
	case req := <-testClient.RequestCh:
		assert.EqualValues(t, req.Header, map[string][]string{
			"Content-Type":     {"application/json"},
			"Test_key":         {"test_value"},
			"User-Agent":       {"user-agent"},
			"X-License-Key":    {""},
			"X-Nri-Entity-Key": {"display-name"},
		})
	case <-time.After(timeout):
		assert.FailNow(t, "timeout while waiting for a response")
	}
	a.Terminate()
}

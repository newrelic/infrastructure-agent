package dm

import (
	"encoding/json"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo/hostname"
)

func BenchmarkNewIntegrationEmitter_10(b *testing.B) {
	const entityCount = 10
	benchmarkSend(b, entityCount)
}

func BenchmarkNewIntegrationEmitter_100(b *testing.B) {
	const entityCount = 100
	benchmarkSend(b, entityCount)
}

func BenchmarkNewIntegrationEmitter_500(b *testing.B) {
	const entityCount = 500
	benchmarkSend(b, entityCount)
}

func BenchmarkNewIntegrationEmitter_1000(b *testing.B) {
	const entityCount = 1000
	benchmarkSend(b, entityCount)
}

func benchmarkSend(b *testing.B, entityCount int) {
	// Given an agent has an identity
	agentIdentity := entity.Identity{
		ID: entity.ID(1337),
	}
	// And we have already registered entities
	registeredEntities := make(RegisteredEntitiesNameToID, entityCount)
	for i := 0; i < entityCount; i++ {
		entityID := fmt.Sprintf("entity_%v", i)
		registeredEntities[entityID] = entity.ID(i)
	}

	// And with DM sender disabled (does not harvest)
	dmSender, err := NewDMSender(MetricsSenderConfig{
		LicenseKey:       "LicenseKey",
		MetricApiURL:     "localhost",
		SubmissionPeriod: 0,
	}, http.DefaultTransport, func() entity.Identity {
		return agentIdentity
	})
	require.NoError(b, err)

	// When we create a new emitter
	dmEmitter := NewEmitter(&noopAgentContext{
		// Setup hostname
		lookUp: map[string]string{
			sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "test",
		},
		// Setup Agent ID
		identity: agentIdentity,
	}, dmSender, &noopRetriever{}, &stubIDProviderInterface{
		registeredEntities: registeredEntities,
	})

	// And payload
	cannedDuration, _ := time.ParseDuration("1m7s")
	cannedDurationInt := int64(cannedDuration.Seconds() * 1000)
	cannedDate := time.Date(1980, time.January, 12, 1, 2, 0, 0, time.Now().Location())
	cannedDateUnix := cannedDate.Unix()
	d := protocol.DataV4{
		DataSets: make([]protocol.Dataset, entityCount),
	}

	d.Integration = protocol.IntegrationMetadata{
		Name:    "benchmark",
		Version: "99",
	}
	for i := 0; i < entityCount; i++ {
		entityID := fmt.Sprintf("entity_%v", i)
		d.DataSets[i] = protocol.Dataset{
			Entity: protocol.Entity{
				Name: entityID,
			},
			Common: protocol.Common{
				Attributes: map[string]interface{}{
					"common_one_key": "common_one_value",
					"port":           8080,
					"ip":             "127.0.0.1",
				},
			},
			Metrics: []protocol.Metric{
				{
					Name:       "GaugeMetric",
					Type:       "gauge",
					Value:      json.RawMessage("1.45"),
					Timestamp:  &cannedDateUnix,
					Attributes: map[string]interface{}{"att_key": "att_value_gauge", "att_key_int": 1.14},
				},
				{
					Name:       "CountMetric",
					Type:       "count",
					Value:      json.RawMessage("2.45"),
					Timestamp:  &cannedDateUnix,
					Interval:   &cannedDurationInt,
					Attributes: map[string]interface{}{"att_key": "att_value_count", "att_key_int": 1.24},
				},
				{
					Name:       "SummaryMetric",
					Type:       "summary",
					Attributes: map[string]interface{}{"att_key": "att_value_summary", "att_key_int": 1.34},
					Timestamp:  &cannedDateUnix,
					Interval:   &cannedDurationInt,
					Value:      json.RawMessage("{ \"count\": 1, \"sum\": 2, \"min\":3, \"max\":4 }"),
				},
			},
		}
	}

	marshal, err := json.Marshal(d)
	require.NoError(b, err)

	extraLabels := map[string]string{
		"extra_labels": "one",
		"some_pod":     "awesome",
		"app/id":       "1337",
		"app/name":     "app name",
		"app/version":  "1.2.3",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Then the emitter should send correctly five times
		for count := 0; count < 5; count++ {
			if err := dmEmitter.Send(integration.Definition{}, extraLabels, []data.EntityRewrite{}, marshal); err != nil {
				b.Error(err)
				b.FailNow()
			}
			// todo: Trigger harvest
		}
	}
}

type stubIDProviderInterface struct {
	registeredEntities RegisteredEntitiesNameToID
}

func (s stubIDProviderInterface) Entities(_ entity.Identity, entities []protocol.Entity) (registeredEntities RegisteredEntitiesNameToID, unregisteredEntities UnregisteredEntities) {
	registeredEntities = make(RegisteredEntitiesNameToID, len(entities))
	for i := range entities {
		registeredEntities[entities[i].Name] = s.registeredEntities[entities[i].Name]
	}
	return
}

type noopRetriever struct {
}

func (n noopRetriever) GetFeatureFlag(name string) (enabled, exists bool) {
	return true, true
}

type noopAgent struct {
}

func (n noopAgent) GetContext() agent.AgentContext {
	return &noopAgentContext{}
}

type noopAgentContext struct {
	lookUp   agent.IDLookup
	identity entity.Identity
}

func (n noopAgentContext) SendData(output agent.PluginOutput) {
	panic("implement me")
}

func (n noopAgentContext) SendEvent(event sample.Event, entityKey entity.Key) {
	panic("implement me")
}

func (n noopAgentContext) Unregister(id ids.PluginID) {
	panic("implement me")
}

func (n noopAgentContext) AddReconnecting(plugin agent.Plugin) {
	panic("implement me")
}

func (n noopAgentContext) Reconnect() {
	panic("implement me")
}

func (n noopAgentContext) Config() *config.Config {
	panic("implement me")
}

func (n noopAgentContext) AgentIdentifier() string {
	panic("implement me")
}

func (n noopAgentContext) Version() string {
	panic("implement me")
}

func (n noopAgentContext) CacheServicePids(source string, pidMap map[int]string) {
	panic("implement me")
}

func (n noopAgentContext) GetServiceForPid(pid int) (service string, ok bool) {
	panic("implement me")
}

func (n noopAgentContext) ActiveEntitiesChannel() chan string {
	panic("implement me")
}

func (n noopAgentContext) HostnameResolver() hostname.Resolver {
	panic("implement me")
}

func (n noopAgentContext) IDLookup() agent.IDLookup {
	return n.lookUp
}

func (n noopAgentContext) AgentIdentity() entity.Identity {
	return n.identity
}

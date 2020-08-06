// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package emitter

import (
	"errors"
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/agent/mocks"
	"strings"
	"testing"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/internal/agent/cmdchannel/handler"
	"github.com/newrelic/infrastructure-agent/internal/feature_flags"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/newrelic/infrastructure-agent/pkg/entity"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/dm"
	"github.com/newrelic/infrastructure-agent/pkg/integrations/v4/protocol"
	"github.com/newrelic/infrastructure-agent/pkg/plugins/ids"
	"github.com/newrelic/infrastructure-agent/pkg/sysinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const integrationJsonOutput = `{
  "name": "%s",
  "protocol_version": "3",
  "integration_version": "1.4.0",
  "data": [
    {
      "metrics": [
        {
          "cluster.connectedSlaves": 0,
          "cluster.role": "slave",
          "db.aofLastBgrewriteStatus": "ok",
          "db.aofLastRewriteTimeMiliseconds": -1,
          "db.aofLastWriteStatus": "ok",
          "db.evictedKeysPerSecond": 0,
          "db.expiredKeysPerSecond": 0,
          "db.keyspaceHitsPerSecond": 0,
          "db.keyspaceMissesPerSecond": 0,
          "db.latestForkMilliseconds": 0,
          "db.rdbBgsaveInProgress": 0,
          "db.rdbChangesSinceLastSave": 0,
          "db.rdbLastBgsaveStatus": "ok",
          "db.rdbLastBgsaveTimeMilliseconds": -1,
          "db.rdbLastSaveTime": 1582018453,
          "db.syncFull": 0,
          "db.syncPartialErr": 0,
          "db.syncPartialOk": 0,
          "event_type": "RedisSample",
          "net.blockedClients": 0,
          "net.clientBiggestInputBufBytes": 0,
          "net.clientLongestOutputList": 0,
          "net.commandsProcessedPerSecond": 0,
          "net.connectedClients": 2,
          "net.connectionsReceivedPerSecond": 0,
          "net.inputBytesPerSecond": 0,
          "net.outputBytesPerSecond": 0,
          "net.pubsubChannels": 0,
          "net.pubsubPatterns": 0,
          "net.rejectedConnectionsPerSecond": 0,
          "port": "6379",
          "software.uptimeMilliseconds": 101096000,
          "software.version": "3.0.3",
          "system.memFragmentationRatio": 2.63,
          "system.usedCpuSysChildrenMilliseconds": 0,
          "system.usedCpuSysMilliseconds": 58260,
          "system.usedCpuUserChildrenMilliseconds": 0,
          "system.usedCpuUserMilliseconds": 19580,
          "system.usedMemoryBytes": 835616,
          "system.usedMemoryLuaBytes": 36864,
          "system.usedMemoryPeakBytes": 872464,
          "system.usedMemoryRssBytes": 2195456
        },
        {
          "event_type": "RedisKeyspaceSample",
          "port": "6379"
        }
      ],
      "inventory": {
        "activerehashing": {
          "value": "yes"
        },
        "aof-load-truncated": {
          "value": "yes"
        },
        "aof-rewrite-incremental-fsync": {
          "value": "yes"
        },
        "appendfsync": {
          "value": "everysec"
        },
        "appendonly": {
          "value": "no"
        },
        "auto-aof-rewrite-min-size": {
          "value": "67108864"
        },
        "auto-aof-rewrite-percentage": {
          "value": "100"
        },
        "bind": {
          "value": ""
        },
        "client-output-buffer-limit": {
          "normal-hard-limit": "0",
          "normal-soft-limit": "0",
          "normal-soft-seconds": "0",
          "pubsub-hard-limit": "33554432",
          "pubsub-soft-limit": "8388608",
          "pubsub-soft-seconds": "60",
          "raw-value": "normal 0 0 0 slave 268435456 67108864 60 pubsub 33554432 8388608 60",
          "slave-hard-limit": "268435456",
          "slave-soft-limit": "67108864",
          "slave-soft-seconds": "60"
        },
        "cluster-migration-barrier": {
          "value": "1"
        },
        "cluster-node-timeout": {
          "value": "15000"
        },
        "cluster-require-full-coverage": {
          "value": "yes"
        },
        "cluster-slave-validity-factor": {
          "value": "10"
        },
        "config-file": {
          "value": ""
        },
        "daemonize": {
          "value": "no"
        },
        "databases": {
          "value": "16"
        },
        "dbfilename": {
          "value": "dump.rdb"
        },
        "dir": {
          "value": "/data"
        },
        "hash-max-ziplist-entries": {
          "value": "512"
        },
        "hash-max-ziplist-value": {
          "value": "64"
        },
        "hll-sparse-max-bytes": {
          "value": "3000"
        },
        "hz": {
          "value": "10"
        },
        "latency-monitor-threshold": {
          "value": "0"
        },
        "list-max-ziplist-entries": {
          "value": "512"
        },
        "list-max-ziplist-value": {
          "value": "64"
        },
        "logfile": {
          "value": ""
        },
        "loglevel": {
          "value": "notice"
        },
        "lua-time-limit": {
          "value": "5000"
        },
        "masterauth": {
          "value": "(omitted value)"
        },
        "maxclients": {
          "value": "10000"
        },
        "maxmemory": {
          "value": "0"
        },
        "maxmemory-policy": {
          "value": "noeviction"
        },
        "maxmemory-samples": {
          "value": "5"
        },
        "mem-allocator": {
          "value": "jemalloc-3.6.0"
        },
        "min-slaves-max-lag": {
          "value": "10"
        },
        "min-slaves-to-write": {
          "value": "0"
        },
        "no-appendfsync-on-rewrite": {
          "value": "no"
        },
        "notify-keyspace-events": {
          "value": ""
        },
        "pidfile": {
          "value": "/var/run/redis.pid"
        },
        "port": {
          "value": "6379"
        },
        "rdbchecksum": {
          "value": "yes"
        },
        "rdbcompression": {
          "value": "yes"
        },
        "redis_version": {
          "value": "3.0.3"
        },
        "repl-backlog-size": {
          "value": "1048576"
        },
        "repl-backlog-ttl": {
          "value": "3600"
        },
        "repl-disable-tcp-nodelay": {
          "value": "no"
        },
        "repl-diskless-sync": {
          "value": "no"
        },
        "repl-diskless-sync-delay": {
          "value": "5"
        },
        "repl-ping-slave-period": {
          "value": "10"
        },
        "repl-timeout": {
          "value": "60"
        },
        "requirepass": {
          "value": "(omitted value)"
        },
        "save": {
          "value": ""
        },
        "set-max-intset-entries": {
          "value": "512"
        },
        "slave-priority": {
          "value": "100"
        },
        "slave-read-only": {
          "value": "yes"
        },
        "slave-serve-stale-data": {
          "value": "yes"
        },
        "slaveof": {
          "value": "redis-master 6379"
        },
        "slowlog-log-slower-than": {
          "value": "10000"
        },
        "slowlog-max-len": {
          "value": "128"
        },
        "stop-writes-on-bgsave-error": {
          "value": "yes"
        },
        "tcp-backlog": {
          "value": "511"
        },
        "tcp-keepalive": {
          "value": "0"
        },
        "timeout": {
          "value": "0"
        },
        "unixsocket": {
          "value": ""
        },
        "unixsocketperm": {
          "value": "0"
        },
        "watchdog-period": {
          "value": "0"
        },
        "zset-max-ziplist-entries": {
          "value": "128"
        },
        "zset-max-ziplist-value": {
          "value": "64"
        }
      },
      "events": []
    }
  ]
}
`

const integrationJsonV4Output = `
{
  "protocol_version": "4",
  "integration": {
    "name": "my.integration.name",
    "version": "integration version"
  },
  "data": [
    {
      "common": {
        "timestamp": 1531414060739,
        "interval.ms": 10000,
        "attributes": {}
      },
      "metrics":[
        {
          "name": "redis.metric1",
          "type": "count",
          "value": 93,
          "attributes": {
            "foo": "bar"
          }
        }
      ],
      "entity": {
        "name": "unique name",
        "type": "RedisInstance",
        "displayName": "human readable name",
        "tags": {
          "foo": "bar"
        }
      },
      "inventory": {
        "foo": {
          "value": "bar"
        }
      },
      "events":[
        {
          "summary": "foo"
        }
      ]
    }
  ]
}
`

type mockDmEmitter struct {
	mock.Mock
}

func (m *mockDmEmitter) Send(
	metadata integration.Definition,
	extraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationJSON []byte) error {

	args := m.Called(metadata, extraLabels, entityRewrite, integrationJSON)
	return args.Error(0)
}

func TestLegacy_Emit(t *testing.T) {
	type testCase struct {
		name                  string
		metadata              integration.Definition
		integrationJsonOutput string
		expectedId            ids.PluginID
	}

	cases := []testCase{
		{
			name: "Inventory source set",
			metadata: integration.Definition{
				InventorySource: *ids.NewPluginID("cat", "term"),
			},
			integrationJsonOutput: integrationJsonOutput,
			expectedId:            *ids.NewPluginID("cat", "term"),
		},
		{
			name: "Inventory source set - protocol V4",
			metadata: integration.Definition{
				InventorySource: *ids.NewPluginID("cat", "term"),
			},
			integrationJsonOutput: integrationJsonV4Output,
			expectedId:            *ids.NewPluginID("cat", "term"),
		},
		{
			name: "Plugin data name",
			metadata: integration.Definition{
				InventorySource: ids.EmptyInventorySource,
			},
			integrationJsonOutput: fmt.Sprintf(integrationJsonOutput, "com.newrelic.something"),
			expectedId:            ids.NewDefaultInventoryPluginID("com.newrelic.something"),
		},
		{
			name: "Plugin data name - protocol V4",
			metadata: integration.Definition{
				InventorySource: ids.EmptyInventorySource,
			},
			integrationJsonOutput: integrationJsonV4Output,
			expectedId:            ids.NewDefaultInventoryPluginID("my.integration.name"),
		},
		{
			name: "Metadata data name",
			metadata: integration.Definition{
				InventorySource: ids.EmptyInventorySource,
				Name:            "awesome-plugin",
			},
			integrationJsonOutput: fmt.Sprintf(integrationJsonOutput, ""),
			expectedId:            ids.NewDefaultInventoryPluginID("awesome-plugin"),
		},
		{
			name: "Metadata data name - protocol v4",
			metadata: integration.Definition{
				InventorySource: ids.EmptyInventorySource,
				Name:            "awesome-plugin",
			},
			integrationJsonOutput: strings.Replace(integrationJsonV4Output, "\"name\": \"my.integration.name\",", "", 1),
			expectedId:            ids.NewDefaultInventoryPluginID("awesome-plugin"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extraLabels := data.Map{}
			entityRewrite := []data.EntityRewrite{}
			integrationJSON := []byte(tc.integrationJsonOutput)

			ma := mockAgent()
			mockDME := &mockDmEmitter{}
			mockDME.On("Send",
				tc.metadata,
				extraLabels,
				entityRewrite,
				integrationJSON,
			).Return(nil)

			em := &Legacy{
				Context:     ma,
				FFRetriever: feature_flags.NewManager(map[string]bool{handler.FlagProtocolV4: true}),
				dmEmitter:   mockDME,
			}

			err := em.Emit(tc.metadata, extraLabels, entityRewrite, integrationJSON)
			require.NoError(t, err)

			for c := range ma.Calls {
				called := ma.Calls[c]
				if called.Method == "SendData" {
					//t.Log(called)
					po := called.Arguments[0].(agent.PluginOutput)
					assert.Equal(t, tc.expectedId, po.Id)
				}
			}
		})
	}
}

func TestProtocolV4_Emit(t *testing.T) {
	metadata := integration.Definition{
		InventorySource: *ids.NewPluginID("cat", "term"),
	}
	extraLabels := data.Map{
		"label.foo":                "bar",
		"extraAnnotationAttribute": "annotated",
	}
	entityRewrite := []data.EntityRewrite{}
	integrationJSON := []byte(integrationJsonV4Output)

	ma := mockAgent()
	mockedMetricsSender := mockMetricSender()

	mockDME := &mockDmEmitter{}
	mockDME.On("Send",
		metadata,
		extraLabels,
		entityRewrite,
		integrationJSON,
	).Return(nil)

	em := &Legacy{
		Context:     ma,
		FFRetriever: feature_flags.NewManager(map[string]bool{handler.FlagProtocolV4: true}),
		dmEmitter:   mockDME,
	}

	err := em.Emit(metadata, extraLabels, entityRewrite, integrationJSON)
	require.NoError(t, err)

	for c := range ma.Calls {
		called := ma.Calls[c]

		if called.Method == "SendData" {
			//t.Log(called)
			pluginOutput := called.Arguments[0].(agent.PluginOutput)
			assert.Equal(t, "unique name", pluginOutput.EntityKey)
			assert.Equal(t, "labels/foo", pluginOutput.Data[1].(protocol.InventoryData)["id"])
			assert.Equal(t, "bar", pluginOutput.Data[1].(protocol.InventoryData)["value"])
		}

		if called.Method == "SendEvent" {
			//t.Log(called)
			entityKey := called.Arguments[1].(entity.Key)
			assert.Equal(t, "unique name", entityKey.String())
		}
	}

	for c := range mockedMetricsSender.Calls {
		called := mockedMetricsSender.Calls[c]

		if called.Method == "SendMetrics" {
			//t.Log(called)
			metrics := called.Arguments[0].([]protocol.Metric)
			assert.Equal(t, 1, len(metrics))
		}
	}
}

func TestProtocolV4_Emit_WithFFDisabled(t *testing.T) {
	metadata := integration.Definition{
		InventorySource: *ids.NewPluginID("cat", "term"),
	}
	extraLabels := data.Map{
		"label.foo":                "bar",
		"extraAnnotationAttribute": "annotated",
	}
	entityRewrite := []data.EntityRewrite{}
	integrationJSON := []byte(integrationJsonV4Output)

	ma := mockAgent()
	mockDME := &mockDmEmitter{}
	mockDME.On("Send",
		metadata,
		extraLabels,
		entityRewrite,
		integrationJSON,
	).Return(errors.New("something failed"))

	em := &Legacy{
		Context:     ma,
		FFRetriever: feature_flags.NewManager(map[string]bool{handler.FlagProtocolV4: false}),
		dmEmitter:   mockDME,
	}

	err := em.Emit(metadata, extraLabels, entityRewrite, integrationJSON)
	require.Error(t, err)
}

func mockAgent() *mocks.AgentContext {
	aID := agent.IDLookup{
		sysinfo.HOST_SOURCE_HOSTNAME:       "long",
		sysinfo.HOST_SOURCE_HOSTNAME_SHORT: "short",
	}

	cfg := &config.Config{
		SupervisorRefreshSec: 1,
		SupervisorRpcSocket:  "/tmp/supervisor.sock.test",
	}

	ma := &mocks.AgentContext{}
	ma.On("AgentIdentifier").Return("bob")
	ma.On("IDLookup").Return(aID)
	ma.On("SendData", mock.AnythingOfType("agent.PluginOutput")).Once()
	ma.On("SendEvent", mock.AnythingOfType("agent.mapEvent"), mock.AnythingOfType("entity.Key")).Once()
	ma.On("Config").Return(cfg)
	ma.On("SendEvent", mock.Anything, entity.Key("bob")).Twice()

	return ma
}

func mockMetricSender() *mockedMetricsSender {
	mockedMetricsSender := &mockedMetricsSender{}
	mockedMetricsSender.On("SendMetrics", mock.AnythingOfType("[]protocol.Metric")).Once()

	return mockedMetricsSender
}

type mockedMetricsSender struct {
	dm.MetricsSender
	mock.Mock
}

func (m *mockedMetricsSender) SendMetrics(metrics []protocol.Metric) {
	m.Called(metrics)
}

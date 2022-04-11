# How payloads in sdk v4 format handled by the agent

When the payload is received from an integration stdout it got processed to determine the protocol version. If the version is v3, we follow the old v3 path, if the version is v4 we have several options. First of all, we need to determine if the register is required for each dataset. All datasets that require registration are sent to the register queue to be registered by the worked and will be sent to the backend once entities are registered.

### Metrics decoration
All metrics sent decorated with common attributes and extra instrumentation attributes:
- instrumentation.version
- instrumentation.name
- instrumentation.provider
- collector.name
- collector.version

### Registration process
| ignore_entity | entity field | result |
|---|---|---|
| ✅ true | ✅ present | send data directly to the backend for an entity synthesis  |
| ❌ false | ✅ present | register entity via `/register` endpoint and then send metrics with attached entity ID  |
| ❌ false | ❌ omitted | send metric attached to the host entity  |

### JSON protocol v4 sample

```json
{
  "protocol_version":"4",                      # protocol version number
  "integration":{                              # this data will be added to all metrics and events as attributes,                                               
                                               # and also sent as inventory
    "name":"integration name",
    "version":"integration version"
  },
  "data":[                                    # List of objects containing entities, metrics, events and inventory
    {
      "ignore_entity": false,                 # tells agent to skip register for this payload
      "entity":{                              # this object is optional. If it's not provided, then the Entity will get 
                                              # the same entity ID as the agent that executes the integration. 
        "name":"redis:192.168.100.200:1234",  # unique entity name per customer account
        "type":"RedisInstance",               # entity's category
        "displayName":"my redis instance",    # human readable name
        "metadata":{}                         # can hold general metadata or tags. Both are key-value pairs that will 
                                              # be also added as attributes to all metrics and events
      },
      "metrics":[                             # list of metrics using the dimensional metric format
        {
          "name":"redis.metric1",
          "type":"count",                     # gauge, count, summary, cumulative-count, rate or cumulative-rate
          "value":93, 
          "attributes":{}                     # set of key-value pairs that define the dimensions of the metric
        }
      ],
      "common":{...}                          # Map of dimensions common to every entity metric. Only string supported.
      "inventory":{...},                      # Inventory remains the same
      "events":[...]                          # Events remain the same
    }
  ]
}
```
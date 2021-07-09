# Status API

New local read-only HTTP JSON API in the agent to provide *status reports*.

As of now *status reports* only contain backend endpoints connectivity checks.

> When a proxy setup is configured for the agent, reachability checks will make use of it.

It requires to be enabled via `status_server_enabled: true` also port is configurable via `status_server_port`.

When enabled by default these *endpoints* will be available locally:
- `http://localhost:8003/v1/status`
- `http://localhost:8003/v1/status/errors`
- `http://localhost:8003/v1/status/entity`

## JSON response shape

### Report

*Endpoint:* `/v1/status`

```json
{
  "checks": {
    "endpoints": [
      {
        "url": "<url>",
        "reachable": false,
        "error": "<optional error  msg>"
      }
    ]
  },
  "config": {
    "reachability_timeout": "<duration>"
  }
}
```

### Report Errors

*Endpoint:* `/v1/status/errors`

Same as above, but:
- *filters out non errored data*
- no errors at all will return an empty object to ease error handling

#### Response with status ok:

```json
{}
```

*Status code:* 201  (created)

#### Response with errored status:

```json
{
  "checks": {
    "endpoints": [
      {
        "url": "https://infra-api.newrelic.com/infra/v2/metrics",
        "reachable": false,
        "error": "endpoint check timeout exceeded, Head \"https://infra-api.newrelic.com/infra/v2/metrics\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
      }
    ]
  },
  "config": {
    "reachability_timeout": "10s"
  }
}
```

*Status code:* 200

#### Errored response:

Status checks couldn't be reported.

*Status code:* 5XX

Empty response body.

### Readiness

*Endpoint:* `/v1/status/ready`

It returns `200` when status API is ready to handle requests.

### Report Entity

*Endpoint:* `/v1/status/entity`

Returns information about the agent/host entity.

#### Response

##### 204

A response status code *204* ("No Content") will be returned when the agent still has no information
about the agent/host entity.

Therefore, it may take several requests to until the agent provides entity data. 

> According to [RFC-2616](https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html) a 204 response
> won't provide any body contents.

##### 200

Entity data gets successfully reported. Body JSON shape:

```json
{
    "guid": "ENTITY_GUID"
}
```

##, Usage

### Setup

Enable status API:
- via config file `status_server_enabled: true`
- or environment variable: `NRIA_STATUS_SERVER_ENABLED=true`.

### Request status report

Once agent starts, wait for status API to be ready.  An INFO entry `Status server started.` will show up in the output/log, which could take ~10s.

Query from the same host: `curl http://localhost:8003/v1/status`

Results: 

```json
{
  "endpoints": [
    {
      "url": "https://staging-identity-api.newrelic.com/identity/v1",
      "reachable": true
    },
    {
      "url": "https://staging-infrastructure-command-api.newrelic.com/agent_commands/v1/commands",
      "reachable": true
    },
    {
      "url": "https://staging-infra-api.newrelic.com/infra/v2/metrics",
      "reachable": true
    },
    {
      "url": "https://staging-infra-api.newrelic.com/inventory",
      "reachable": true
    },
    {
      "url": "https://staging-metric-api.newrelic.com/metric/v1/infra",
      "reachable": false,
      "error": "endpoint check timeout exceeded, Head \"https://staging-metric-api.newrelic.com/metric/v1/infra\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
    }
  ],
  "config": {
    "reachability_timeout": "10s"
  }
}
```

### Request entity status report

Query agent/host entity status: `curl http://localhost:8003/v1/entity`

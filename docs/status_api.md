# Status API

New local read-only HTTP JSON API in the agent to provide *status reports*.

As of now *status reports* only contain backend endpoints connectivity checks.

> When a proxy setup is configured for the agent, reachability checks will make use of it.

It requires to be enabled via `status_server_enabled: true` also port is configurable via `status_server_port`.

When enabled by default these *endpoints* will be available locally:
- `http://localhost:8003/v1/status`
- `http://localhost:8003/v1/status/errors`

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
    ],
  },
  "config": {
    "reachability_timeout": "<duration>"
  }
}
```

### ReportErrors

*Endpoint:* `/v1/status/errors`

Same as above, but:
- *filters out non errored data*
- no errors at all will return an empty object to ease error handling

### Response with status ok:

```json
{}
```

*Status code:* 201  (created)

### Response with errored status:

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

### Errored response:

Status checks couldn't be reported.

*Status code:* 5XX

Empty response body.


## Usage

### Setup

Enable status API:
- via config file `status_server_enabled: true`
- or environment variable: `NRIA_STATUS_SERVER_ENABLED=true`.

### Run report

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


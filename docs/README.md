# New Relic infrastructure agent developer documentation

This README provides more detailed information for contribution and issue troubleshooting.

> Installation, configuration, and usage documentation is available at [New Relic Docs](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure).

## Overview

The New Relic infrastructure agent orchestrates data retrieval and data forwarding into the platform. 

Data is gathered through **integrations**. On-host integrations are small binaries than retrieve data from different sources like the OS or external services (NGINX, MySQL, Redis, etc.). Integrations are executed by the **agent** at defined intervals or kept running indefinitely under the agent supervision.

Integrations write their payload into `stdout` and log into `stderr` (for more information, see [integration specs](https://docs.newrelic.com/docs/integrations/integrations-sdk/file-specifications/integration-executable-file-specifications). The agent reads the payloads, performs some processing, and forwards them to the New Relic platform. 

Integrations can produce different [types of 
data](https://docs.newrelic.com/docs/integrations/infrastructure-integrations/get-started/understand-use-data-infrastructure-integrations). For a list of available integrations see the [docs 
site](https://docs.newrelic.com/docs/integrations/host-integrations/host-integrations-list).

To facilitate building your own integrations we provide a [Golang SDK](https://github.com/newrelic/infra-integrations-sdk). 

## Binaries

The New Relic infrastructure agent is composed of different binaries. Besides integrations, there are three executables:

```
├── newrelic-infra
├── newrelic-infra-ctl
└── newrelic-infra-service
```

### `newrelic-infra-service`

This binary is the daemon managed by the service manager (`systemd`, `upstart`, `init-v`).

Its only purpose is to get safe runtime reload/restart. This is achieved by signaling its child `newrelic-infra`. 

### `newrelic-infra`

This binary owns the whole agent runtime. It can be triggered in stand-alone mode if reload/restart features are not required. 

### `newrelic-infra-ctl`

This is the CLI control command to communicate with the agent daemon.

## Runtime steps

There's three different runtime steps:

1. Startup
2. Main runtime
3. Shutdown

### 1. Startup

#### Connectivity check

The agent performs an initial network connection check against New Relic endpoints before bootstrapping the rest of the runtime.

In case of failure, the agent retries connecting to New Relic till the limit of attempts and time is reached. 

#### Connection

This step attempts to uniquely identify the agent/box.

As hostnames are prone to collision and change, a fingerprint is used to present some host/cloud information that might be valuable for identification, such as hostname, cloud instance id, etc.

The agent retrieves the fingerprinting data and requests a unique identifier to the New Relic identity endpoint. Errors at this point behave as circuit breaker, blocking any data submission to the platform.

In case of failure, the agent retries connecting to New Relic till the limit of attempts and time is reached. This step is run concurrently so it avoids blocking the runtime. 

#### 2. Main runtime

The main runtime workflow addresses data processing and submission.

Codebase differentiates different paths for:

- Metrics and events: these share the same workflow, as non dimensional metrics are represented 
through events. See [docs](https://docs.newrelic.com/docs/using-new-relic/data/understand-data/new-relic-data-types)
for further information.
- Inventory: data which might require state to be persisted between agent/box restarts. See 
[docs](https://docs.newrelic.com/docs/infrastructure/infrastructure-ui-pages/infra-ui-pages/infrastructure-inventory-page-search-your-entire-infrastructure)
 for further information.

##### Data sources

**Host metrics** are retrieved by embedded **samplers**, for example: `ProcessSampler, StorageSampler, ...`.

**Host inventory** is retrieved by embedded **inventory plugins**, for example: `KernelModulesPlugin, DpkgPlugin...`

Each type of source has different workflow paths.

**External services data** is retrieved using integrations. Integrations are managed by the `integrations` package. There are different integration protocol versions. Each defines a [JSON API](https://docs.newrelic.com/docs/integrations/infrastructure-integrations/get-started/understand-use-data-infrastructure-integrations).

##### Data processing

Metrics/Events:

- Event queue is shared for all the events (agents and integrations).
- When event-queue reaches 1K events, the agent discard new events. In this case it logs this error message: `Could not queue event: Queue is full..`
  > We already know this is not optimal and that it should change once we add agent-level rate-limiting.
- Therefore, the agent won't ensure data is reported.
  > This will change with the feature mentioned above.
- Events from the queue are batched (default batch queue size is 200).
  > Does this mean that the agent could do 200 batches, each with 1000 rows?
    - Batching and queueing run in parallel, so it could happen that while the batcher is feeding and sending batches, the event-queue reaches its capacity.
    - So you couldn't say there's a total limit of 200*1K.

Integrations:

- They are started concurrently at similar times.
  * At the first run there is a random delay between 0 and their defined interval, which is used in 
  order to spread the load.
  * For subsequents runs their defined interval is used.
- There's no mechanism for waiting on other plugins/instances completion between runs.

#### 3. Shutdown
 
Shutdown is handled by both `newrelic-infra-service` and `newrelic-infra`. `newrelic-infra-service` is called by the OS service manager, forwarding this request to `newrelic-infra`, which receives notifications about shutdown via signaling on Linux and using named-pipes on Windows.

The agent attempts to gracefully shutdown its children processes (integrations) and go-routines. There's a grace time period which, once reached, executes a force stop. 

The agent differentiates between OS shutdown and agent service stop. This allows avoiding triggering alerts on cloud scheduled instances decommision (for example, when downscaling).

## Tests

We differentiate `harvest` tests from the usual ones. The prior assert data retrieval from the underlying OS, whereas the latter are expected to not be coupled to the OS. A build-tag is used to run the `harvest` ones.

Usual package tests lie within each pacakge, but behavioural ones lie at `test/` folder. The core logic behavior is covered at `test/core` using fixtures to replace data retrieval. 

> Currently, not all the available test suites are run in the public CI (Github actions). A private CI is still used while the CI migration into public GHA is accomplished.

There are also some special test suites covering:

- Performance benchmarks
- Fuzz testing
- Proxy end-to-end behaviour
 
## Containerised agent

The recommended way to run the agent as a container is to use the [infrastructure-bundle](https://github.com/newrelic/infrastructure-bundle/), which 
contains not only the agent but also all the official, up-to-date integrations.

Releases are accessible [here](https://github.com/newrelic/infrastructure-bundle/releases). Within the same repository you can find information to manually build the container, so you can customize it. 

> The Kubernetes integration is not included in the bundle as it's deployed via [manifest](https://docs.newrelic.com/docs/integrations/kubernetes-integration/installation/kubernetes-integration-install-configure).

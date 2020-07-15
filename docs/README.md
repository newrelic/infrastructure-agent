# NR infrastructure agent documentation

## User documentation

Agent installation, configuration and usage documentation is available at [New Relic 
docs](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure).

Overall New Relic infrastructure product is available at [New Relic infrastructure 
docs](https://docs.newrelic.com/docs/infrastructure).

Documentation at this repo aims covering more detailed information useful for contribution and issue
 troubleshooting.


## Responsibilities

NR infrastructure agent responsibility is to orchestrate data retrieval and data forwarding 
into the platform.

### Data retrieval

NR infrastructure on-host agent gathers data from **integrations**.


#### Integrations

On-host integrations are small binaries than retrieve data from different sources like OS or 
external services (like Nginx, Redis...).

They are executed by the **agent** at defined intervals or kept running indefinitely under agent 
supervision.

Integrations write their payload into `stdout` and log into `stderr`. See [integration 
specs](https://docs.newrelic.com/docs/integrations/integrations-sdk/file-specifications/integration-executable-file-specifications).
 Agent reads these, performs some processing and forward their payload into New Relic platform. 

They produce different [types of 
data](https://docs.newrelic.com/docs/integrations/infrastructure-integrations/get-started/understand-use-data-infrastructure-integrations).


There a list of available integrations at the [docs 
site](https://docs.newrelic.com/docs/integrations/host-integrations/host-integrations-list)

To ease building your own integrations we provide a [Golang 
SDK](https://github.com/newrelic/infra-integrations-sdk). 


## Binaries

NR infrastructure agent is composed of different binaries. Besides integrations, there are 3 main 
ones:

```
├── newrelic-infra
├── newrelic-infra-ctl
└── newrelic-infra-service
```

### newrelic-infra-service

This binary is the daemon managed by the service manager (systemd, upstart, init-v).

It's only purpose is to get safe runtime reload/restart. This is achieved by signaling its child
`newrelic-infra`. 

### newrelic-infra

This binary owns the whole agent runtime.

It could be triggered in stand-alone mode if reload/restart features are not required.  

### newrelic-infra-ctl

This is the CLI control command to communicate with the agent daemon.


## Runtime

We could describe 3 different runtime steps:

- Startup
- Main runtime
- Shutdown

### Startup

#### Connectivity check

Agent performs an initial network connection check against NR endpoints before bootstrapping the 
rest of the runtime.

> Failures will be retried with a maximum limit of attempts and time.  

#### Connect

This step attempts to uniquely identify the agent/box.

As hostnames are prone to collision and change a *fingerprint* is used to present some host/cloud 
information that might be valuable for identification, such as hostname, cloud instance id and so.

Agent retrieves this *fingerprint* data and requests a unique identifier from New Relic identity 
endpoint.

Errors here behave as circuit breaker blocking any data submission to the platform.

> Failures will be retried with a maximum limit of attempts and time.
>
> This step is run concurrently so it avoids blocking the runtime. 


#### Main runtime

Main runtime workflow addresses data processing and submission.

Codebase differentiates different paths for:

- metrics and events: these share the same workflow, as non dimensional metrics are represented 
through events.
- inventory: stateful data, might require disk persistence.

##### Data sources

**Host metrics** are retrieved by embeded **samplers**, ie: `ProcessSampler, StorageSampler, ...`.

**Host inventory** is retrieved by embeded **inventory plugins**, ie: `KernelModulesPlugin, 
DpkgPlugin...`

Each one have different workflow paths.

**External services data** is retrieved using integrations. Integrations are managed at the 
`integrations` package. There are different *integration protocol* versions. Each one define a 
[JSON API](https://docs.newrelic.com/docs/integrations/infrastructure-integrations/get-started/understand-use-data-infrastructure-integrations).


##### Data processing

Metrics/Events:

- Event queue is shared for all the events (agent's and integrations)
- When event-queue reaches 1K events agent discard new events. In this case it'll log this error msg `Could not queue event: Queue is full..`
  * We already know this is not optimal and it should change once we add agent-level rate-limiting
- Therefor agent won't ensure data is reported
  * This will change with the feature mentioned above
- Events from the queue are batched (default batch queue size 200)
  * This means agent could do 200 batches, each with 1000 rows?
    - Batching and queueing run in parallel, so it could happen that while batcher is feeding and sending batches the event-queue reaches its capacity.
    - So you couldn't say there's a total limit of 200*1K

Integrations:

- They are started concurrently at similar times.
  * There is a random delay btw 0 and their defined interval is used in order to spread the load.
  * For subsequents runs their defined interval is used.
- There's no mechanism for waiting on other plugins/instances completion btw runs.

#### Shutdown
 
Shutdown is handled by both `newrelic-infra-service` and `newrelic-infra`. The former is called by 
OS service manager forwarding this request into the later. Later receives notifications about 
shutdown via signaling on Linux and using named-pipes on Windows.

Agent attempts to gracefully shutdown its children processes (integrations) and go-routines. There's
 a grace time period, once reached force stop is executed. 

Agent differentiates between OS shutdown and agent service stop. This allows to avoid triggering 
alerts on cloud scheduled instances decommision, for instance when downscaling.


## Tests

We differentiate `harvest` tests from the usual ones. The prior require are aimed to assert data 
retrieval from underlying OS, whereas the laters are expected to not be coupled to the underlying 
OS. A build-tag is used to run the `harvest` ones.

Usual package tests lie within each pacakge but behavioural ones lie at `test/` folder.

The core logic behaviour is covered at `test/core` using fixtures to replace data retrieval. 


> At the moment not all the available test suites are run in the public CI (Github actions).
> A private CI is still used while the CI migration into public GHA is accomplished.

There are also some special test suites covering:
- performance benchmarks
- fuzz testing
- proxy end-to-end behaviour
 
## Containerised agent

The recommended way to run the agent as a container is to use the **[infrastructure-bundle](https://github.com/newrelic/infrastructure-bundle/)** which 
contains not only the agent but all the official integrations up to date.

Releases are available at [GH releases](https://github.com/newrelic/infrastructure-bundle/releases).

Within the same repository there's information to manually build the container, so you can customize
 it.

Kubernetes integration is not included in the *bundle* as it's delployed via 
[manifest](https://docs.newrelic.com/docs/integrations/kubernetes-integration/installation/kubernetes-integration-install-configure).

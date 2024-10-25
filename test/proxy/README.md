# Proxy tests architecture

The following figure depicts the components that take place in the Proxy testing
environment: 

```
                        Go Tests Battery
restarts     +----------+              +--------------+ cleans up test data
sends env    |                                        | retrieves stored samples
sends config |                                        | retrieves registered proxies
      file   |                                        |
             |                                        |
             |                                        |
             v                                        |
       +-----+----+     direct communication   +------v----+
       |          +---------------------------->           |
       |          |                            |           |
       |minimalist|                            | Fake      |
       |  agent   |                   forwards | Collector |
       |          |         +---------+-------->           |
       |          +--------->  Squid  |        +------^----+
       +----------+  proxy  |  HTTP   |               |
                     commu- |  proxy  |               |
                     nica-  +---------+---------------+
                     tion                     notifies
```

The go tests battery runs as the rest of go tests (but you need to set the `proxytests`
tag), the rest of components run in their own container.

To run the tests:

```
docker compose -f test/proxy/docker-compose.yml up --build
go test --tags=proxytests ./test/proxy/
```

Everytime you run the test with modification on agent or squid configurations or just recreate the docker-compose environment please make sure you delete the docker network, if not you'll see that they stop working properly.
In order to do that either run:

```
docker compose -f test/proxy/docker-compose.yml down
```
Or if you've stopped the containers using `Ctrl+C` then you should run:
```
docker network prune
```

## Components

### Minimalist agent

It's a container with two components:

* A tiny agent executable that only contains a `Fake` sampler to submit data to the
  collector every second.
* An HTTP service that starts, stops and configures the agent executable. It receives
  a configuration file and a set of environment variables and forwards them to the
  agent.

The HTTP service allows controlling the agent from outside the container (the Go tests
battery) without having to start/stop new containers on every test case.

### Fake collector

It's a simple HTTPS service with the following functionalities:

* Receives metrics in the `/metrics/events/bulk` endpoint and stores them in a queue.
* Receives a notification from the proxy every time the proxy is going to forward
  data to it.
* Allows retrieving the received metrics and proxy notifications from the Go tests
  battery.
* Allows being cleaned up on every new tests (all the stored data is removed).

For maximum fidelity with a real scenario, it uses a secure connection, so it has
its own certificates (`/cabundle` folder), which have to be shared with the 
minimalist agent.

### Squid proxy

It's an HTTP Squid proxy (no secure configuration), that can redirect both `http`
and `https` requests. The agent can be configured to use it with the `HTTP_PROXY`,
`NRIA_PROXY` and `proxy` configuration options (as `http://http-proxy:3128`) or
with the `HTTPS_PROXY` option (as `http://http-proxy:3128`).

We need to verify in the tests that the traffic goes through this proxy. Modifying
the request body, headers or URL (e.g. to add a token that demonstrates the request
passed by there) is not feasible in HTTPS connections, since all this data must be
ciphered with the collector private key.

To workaround this limitation, the `/etc/squid/squid.conf` file has the following
entry:

```
url_rewrite_program /redirector.sh
```

The `redirector.sh` is invoked every time the Proxy gets a request. It does not
do anything that affects the original HTTP request (does not rewrite URLs, does
not redirect traffic). It just submits the name of the proxy to the Fake collector
through an alternative endpoint (so de Collector is aware that there is some
traffic going by this proxy, and can report that to the tests).

### Go tests battery

They run as normal tests in the host, which is able to access to the minimalist
agent and fake collector management endpoints (because the respective containers
expose their ports). The go tests require that the containers are previously running.

Every test usually does the following steps:

* Restarts the agent, submitting the following data:
    - Agent configuration file (if empty, default configuration will be used)
    - Environment variables
* Cleans up the Fake collector (so data from previous tests is removed)
* Ask the collector for the next received sample and verifies the samples
  have the correct information (e.g. the `displayName` coincides with the
  test configuration)
* Ask the collector for the proxy information, which at the moment should be
  `http-proxy` (more identifiers could appear in future tests) or empty if the
  Agent is configured to work without a proxy.

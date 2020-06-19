# New Relic Infrastructure

This Docker image contains the [New Relic Infrastructure](https://newrelic.com/infrastructure) agent capable of monitoring an underlying Docker host.

## Why
Many new operating systems don't provide package managers and exclusively utilize containers for application deployment. This Docker image allows you to deploy the [Infrastructure](https://newrelic.com/infrastructure) agent as a container that will monitor its underlying host.

## Usage

### Pulling
`docker pull newrelic/infrastructure:latest`


### Simple Setup
1. Run the container with the [required run flags](#required-run-flags):

   ```bash
   docker run \
       -d \
       --name newrelic-infra \
       --network=host \
       --cap-add=SYS_PTRACE \
       -v "/:/host:ro" \
       -v "/var/run/docker.sock:/var/run/docker.sock" \
       -e NRIA_LICENSE_KEY="YOUR_LICENSE_KEY" \
       newrelic/infrastructure:latest
   ```

   Replacing `"YOUR_LICENSE_KEY"` with your [license key](https://docs.newrelic.com/docs/accounts-partnerships/accounts/account-setup/license-key).

### Custom Setup (Recommended)
It's recommended that you extend the `newrelic/infrastructure` image and provide your own `newrelic-infra.yml` [agent config](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/configuration/configure-infrastructure-agent) file.

#### Building
1. Create your `newrelic-infra.yml` agent config file, ex.

    ```yaml
    license_key: YOUR_LICENSE_KEY
    ```
    _See the [Infrastructure config docs](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/configuration/configure-infrastructure-agent) for more info._
1. Create your `Dockerfile` extending the `newrelic/infrastructure` image and add your config to `/etc/newrelic-infra.yml`, ex.

    ```bash
    FROM newrelic/infrastructure:latest
    ADD newrelic-infra.yml /etc/newrelic-infra.yml
    ```
1. Build and tag your image, ex.

    ```bash
    docker build -t your-image .
    ```

#### Running
Run the container from the image you built with the [required run flags](#required-run-flags), ex.

```bash
docker run \
    -d \
    --name newrelic-infra \
    --net=host \
    --cap-add=SYS_PTRACE \
    -v "/:/host:ro" \
    -v "/var/run/docker.sock:/var/run/docker.sock" \
    your-image
```

Where `your-image` is the name you tagged your image with.

## Why the required container privileges?
Due to resource isolation from the host and other containers via [Linux namespaces](https://en.wikipedia.org/wiki/Linux_namespaces), by default, a container has a very restricted view and control of its underlying host's resources. Without these extra privileges the [Infrastructure](https://newrelic.com/infrastructure) agent would not be able to monitor the host and its containers.

The [Infrastructure](https://newrelic.com/infrastructure) agent collects data about its host using system files and system calls. For more information about how the [Infrastructure](https://newrelic.com/infrastructure) agent collects data see [Infrastructure and security](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/getting-started/infrastructure-security).

### Required run flags

#### `--network=host`
Sets the container's network namespace to the host's network namespace. This allows the agent to collect the network metrics about the host.

#### `-v "/:/host:ro"`
Bind mounts the host's root volume to the container. This __read-only__ access to the host's root allows the agent to collect process and storage metrics as well as [Inventory](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/infrastructure-ui-pages/infrastructure-inventory-page-search-your-entire-infrastructure) data from the host.

#### `--cap-add=SYS_PTRACE`
Adds the linux capability to trace system processes. This allows the agent to gather data about processes running on the host. Read more [here](https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities)

#### `-v "/var/run/docker.sock:/var/run/docker.sock"`
Bind mounts the host's Docker daemon socket to the container. This allows the agent to connect to the [Engine API](https://docs.docker.com/engine/api/) via the [Docker daemon socket](https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-socket-option) to collect the host's [container data](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/data-instrumentation/docker-instrumentation-infrastructure).

## Changelog

### 0.0.27

#### Added
- NewRelic infrastructure agent runs under a init-process called [Tini](https://github.com/krallin/tini) in order to avoid leaking `defunct` processes when running some integrations like [Cassandra](https://github.com/newrelic/infra-integrations/tree/master/integrations/cassandra).


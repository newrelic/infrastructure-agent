# Containerized Agent

The infrastructure agent support Docker version 1.12  or higher.

## Precondition

Be sure you already [built the agent binaries](https://github.com/newrelic/infrastructure-agent#compile-and-build-the-agent), so these should be available at project root `target/` folder.

## Automatic build and deploy

[`make build/base`](Makefile) builds the Docker image from this repo's [Dockerfile](Dockerfile).

The make target generates the Docker image in steps:

1. Adds the `newrelic-infra` agent binary.
2. Generates and adds a `VERSION` file.
3. Adds all files in [static assets](assets).
4. Sets image labels.
5. Sets image environment variables required for the agent to run correctly inside a container.

> See the comments in [Makefile](Makefile) for the required env vars.

## Manual build and deploy

### Build `newrelic-infra` for Linux

To build the Linux binary, see the [README](../../README.md) instructions. That will produce the `newrelic-infra` binary in `target/bin/`. 

### Build the Docker Image

1. Set the following environment variables:
    * `PROJECT_ROOT`: Path to this cloned repo's root
    * `IMAGE_VERSION`: Version to use for the Docker image
    * `IMAGE_TAG`: `newrelic/infrastructure`
    * `AGENT_VERSION`: Version of the `newrelic-infra` agent
2. Create a workspace directory in the repo root:

    ```bash
    mkdir ${PROJECT_ROOT}/workspace
    ```
3. Copy or move your `newrelic-infra` binary into `${PROJECT_ROOT}/workspace/`. The binary __must__ be named `newrelic-infra`.
4. Run the make target: `make build/base`

This should build the Docker image `newrelic/infrastructure` and tag it with `latest`.

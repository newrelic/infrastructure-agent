# Containerized Agent

The infrastructure agent support Docker version 1.12  or higher.

## Precondition

Be sure you already [built the agent binaries](https://github.com/newrelic/infrastructure-agent#compile-and-build-the-agent), so these should be available at project root `dist/` folder.

## Automatic build and deploy

[`make build/base`](Makefile) builds the Docker image from this repo's [Dockerfile](Dockerfile).

The make target generates the Docker image in steps:

1. Adds the `newrelic-infra` agent binary.
2. Generates and adds a `VERSION` file.
3. Adds all files in [static assets](assets).
4. Sets image labels.
5. Sets image environment variables required for the agent to run correctly inside a container.

> See the comments in [Makefile](Makefile) for the required env vars.

## Manual build

### Build `newrelic-infra` for Linux

To build the Linux binary, see the [README](../../README.md) instructions. That will produce the `newrelic-infra` binary in `dist/`.
Once the binaries for your chose architecture are build you are ready to build the container image.

### Build the Docker Image

1. Set the following environment variables:
    * `PROJECT_ROOT`: Path to this cloned repo's root
    * `IMAGE_VERSION`: Version to use for the Docker image
    * `IMAGE_TAG`: `newrelic/infrastructure`
    * `AGENT_VERSION`: Version of the `newrelic-infra` agent
2. Run the make target: `make build/base`

This should build the Docker image `newrelic/infrastructure` and tag it with `latest`.

### Building the Docker Image locally for other architectures

1. Set the following environment variables:
   * `PROJECT_ROOT`: Path to this cloned repo's root
   * `IMAGE_VERSION`: Version to use for the Docker image
   * `IMAGE_TAG`: `newrelic/infrastructure`
   * `AGENT_VERSION`: Version of the `newrelic-infra` agent
2. Run the make target: `make build/base USE_BUILDX=true DOCKER_ARCH=<OS arch, eg. arm64>`

This should build the Docker image for the target architecture `newrelic/infrastructure` and tag it with `latest`.

### Example building Linux ARM64 image

From the root of the project:

```shell
$ make dist-for-os GOOS=linux GOARCH=arm64
$ make -C build/container/ clean build/base USE_BUILDX=true DOCKER_ARCH=arm64
```

There is a shortcut make target for build ARM images:

* `make build/base-arm64` will build the base image as an `arm64` image
* `make build/base-arm` will build the base image as an `arm` image

## Manually publishing the image

### Publishing the release candidate

To publish all the supported images we can use the following make command (from the root of this project):

```shell
$ make -C build/container/ clean publish/multi-arch-base NS=test REPO=agent AGENT_VERSION=1.2.3
```

This will create all the docker images and tag them as follows, all as "release candidates":

* `arm` as `test/agent:1.2.3-rc-arm`
* `arm64` as `test/agent:1.2.3-rc-arm64`
* `amd64` as `test/agent:1.2.3-rc-amd64`

Setting `NS` sets the Docker organisation to use (defaults to `newrelic`) and `REPO` sets the repo (defaults to `infrastructure`).
The image version is set using `AGENT_VERSION` and should match the one for the agent being added to the image.

This does not actually push the Docker images and manifest to Docker hub.
To do this you need to pass the argument `DOCKER_PUBLISH=true`.

So if you're happy publishing to the default repo then the following will do that:

```shell
$ make -C build/container/ clean publish/multi-arch-base DOCKER_PUBLISH=true AGENT_VERSION=1.2.3
```

### Promoting the release candidate

Once you are happy with the release candidate you can run the following make target to promote the release candidate:

```shell
$ make -C build/container/ publish/multi-arch-base-rc DOCKER_PUBLISH=true AGENT_VERSION=1.2.3
```

Again, if `DOCKER_PUBLISH` not set to `true` then nothing will be published to Docker hub.
Make sure the `AGENT_VERSION` matches the one you used for the release candidate.

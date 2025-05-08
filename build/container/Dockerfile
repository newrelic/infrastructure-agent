ARG base_image=alpine:3.21.3

FROM $base_image AS core

ARG image_version=0.0
ARG agent_version=0.0
ARG version_file=VERSION
ARG agent_bin=newrelic-infra

# Add the agent binary
COPY $agent_bin /usr/bin/newrelic-infra
COPY ${agent_bin}-ctl /usr/bin/newrelic-infra-ctl
COPY ${agent_bin}-service /usr/bin/newrelic-infra-service

# Add all static assets
COPY assets /newrelic

# Add the VERSION file
COPY $version_file /newrelic/VERSION

LABEL com.newrelic.image.version=$image_version \
      com.newrelic.infra-agent.version=$agent_version \
      com.newrelic.maintainer="infrastructure-eng@newrelic.com" \
      com.newrelic.description="New Relic Infrastructure agent for monitoring the underlying host."

ENV NRIA_IS_CONTAINERIZED true
ENV NRIA_OVERRIDE_HOST_ROOT /host

# Upgrade all installed packages to their latest versions
RUN apk --no-cache upgrade && \
    apk add --no-cache \
        c-ares=1.34.5-r0 \
        sqlite-libs=3.48.0-r1 \
        xz-libs=5.6.3-r1

RUN apk add --no-cache --upgrade \
    ca-certificates \
    # Embed required dlls:
    # ldd /usr/bin/newrelic-infra
    #   /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    #   libpthread.so.0 => /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    #   libc.so.6 => /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    # As musl and glibc are compatible, this symlink fixes the missing dependency
    # The simlink is added both for amd64 and arm64 architectures
    && mkdir /lib64 \
    && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2 \
    && ln -s /lib/libc.musl-aarch64.so.1 /lib64/ld-linux-aarch64.so.1 \
    # libresolv.so.2 is needed when CGO is enabled so we add the glibc compatibility for Alpine
    && apk add --no-cache gcompat \
    && apk add --no-cache tini

# Tini is now available at /sbin/tini
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/usr/bin/newrelic-infra-service"]

#################################
# Forwarder
#################################
FROM core AS forwarder

RUN apk add --no-cache \
        curl

RUN addgroup -g 2000 nri-agent && adduser -D -H -u 1000 -G nri-agent nri-agent
USER nri-agent

ENV NRIA_OVERRIDE_HOST_ROOT ""
ENV NRIA_IS_SECURE_FORWARD_ONLY true

#################################
# K8s events forwarder
#################################
FROM forwarder AS k8s-events-forwarder

ENV NRIA_HTTP_SERVER_ENABLED true

#################################
# BASE
#################################
FROM core AS base

ARG nri_pkg_dir
ARG nri_docker_version
ARG nri_flex_version
ARG nri_prometheus_version

LABEL com.newrelic.nri-docker.version=$nri_docker_version \
      com.newrelic.nri-flex.version=$nri_flex_version \
      com.newrelic.nri-prometheus.version=$nri_prometheus_version

RUN apk add --no-cache \
        ntpsec \
        curl

COPY $nri_pkg_dir /

ARG base_image=alpine:3.11

FROM $base_image AS core

ARG image_version
ARG agent_version
ARG version_file
ARG agent_bin

# Add the agent binary
COPY $agent_bin /usr/bin/newrelic-infra
COPY ${agent_bin}-ctl /usr/bin/newrelic-infra-ctl
#COPY ${agent_bin}-service /usr/bin/newrelic-infra-service

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

RUN apk add --no-cache --upgrade \
    ca-certificates \
    # Embed required dlls:
    # ldd /usr/bin/newrelic-infra
    #   /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    #   libpthread.so.0 => /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    #   libc.so.6 => /lib64/ld-linux-x86-64.so.2 (0x7f2bbbd0f000)
    # As musl and glibc are compatible, this symlink fixes the missing dependency
    && mkdir /lib64 \
    && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2 \
    && apk add --no-cache tini

# Tini is now available at /sbin/tini
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["/usr/bin/newrelic-infra"]

#################################
# BASE
#################################
FROM core AS base

ARG nri_pkg_dir
ARG nri_docker_version
ARG nri_flex_version

LABEL com.newrelic.nri-docker.version=$nri_docker_version \
      com.newrelic.nri-flex.version=$nri_flex_version

RUN apk add --no-cache ntpsec=1.1.8-r0
RUN apk add --no-cache curl=7.67.0-r0

COPY $nri_pkg_dir /

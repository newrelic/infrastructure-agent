# Container Build Skill

Build containerized Docker images for the New Relic infrastructure agent.

> **IMPORTANT:** Never run publish or upload commands. Only build images locally. Do not execute any `make` targets that push or publish images to a registry.

## Prerequisites

Agent binaries must be built first and available in `dist/`. Run one of:

```bash
make dist-for-os GOOS=linux
make dist-for-os GOOS=linux GOARCH=arm64
```

## Linux Container Images

**Build base image (amd64, local load):**

```bash
make -C build/container/ build/base AGENT_VERSION=<version>
```

**Build for a specific architecture:**

```bash
# amd64
make -C build/container/ build/base DOCKER_ARCH=amd64 AGENT_VERSION=<version>

# arm64
make dist-for-os GOOS=linux GOARCH=arm64
make -C build/container/ clean build/base USE_BUILDX=true DOCKER_ARCH=arm64 AGENT_VERSION=<version>

# arm
make -C build/container/ clean build/base USE_BUILDX=true DOCKER_ARCH=arm AGENT_VERSION=<version>
```

**Build forwarder image:**

```bash
make -C build/container/ build/forwarder AGENT_VERSION=<version>
```

**Build k8s-events-forwarder image:**

```bash
make -C build/container/ build/k8s-events-forwarder AGENT_VERSION=<version>
```

## Windows Container Images

**Build Windows image (ltsc2022, default):**

```bash
make -C build/container/ build/base-windows AGENT_VERSION=<version>
```

**Build specific Windows version:**

```bash
# Windows Server 2022
make -C build/container/ build/base-windows-ltsc2022 AGENT_VERSION=<version>

# Windows Server 2019
make -C build/container/ build/base-windows-ltsc2019 AGENT_VERSION=<version>
```

## Image Names

| Image | Default tag |
|-------|-------------|
| `newrelic/infrastructure` | base agent image |
| `newrelic/infrastructure-core` | core-only image |
| `newrelic/nri-forwarder` | forwarder |
| `newrelic/k8s-events-forwarder` | Kubernetes events forwarder |

Append `-fips` suffix to `FIPS=-fips` for FIPS-compliant variants:

```bash
make -C build/container/ build/base FIPS=-fips AGENT_VERSION=<version>
```

## Utility

**Clean workspace:**

```bash
make -C build/container/ clean
```

**Lint Dockerfile:**

```bash
make -C build/container/ lint
```

## Notes

- Container Makefile and Dockerfiles live in `build/container/`
- Workspace directory used during build: `build/container/workspace/` (gitignored)
- `USE_BUILDX=true` required for cross-architecture builds
- `DOCKER_PUBLISH=true` required to actually push images; omit to only build locally
- FIPS variants omit `arm` arch (amd64 + arm64 only)

# CLAUDE.md — New Relic Infrastructure Agent

Go agent that collects inventory and metrics from hosts and sends them to the New Relic platform.

Module: `github.com/newrelic/infrastructure-agent`  
Go: 1.25+  
Base branch: `master`  
Vendored: yes — all deps in `vendor/`, do not run `go get` or `go mod download`

---

## Build

```bash
make compile                          # build for current OS
make dist-for-os GOOS=linux           # cross-compile (linux / darwin / windows)
make debug-for-os GOOS=linux          # build with debug symbols
```

Output binaries in `target/bin/{OS_ARCH}/`:

- `newrelic-infra` — main agent process
- `newrelic-infra-service` — parent service process
- `newrelic-infra-ctl` — troubleshooting utility

---

## Test

```bash
make test                             # all unit tests with race detector (30m timeout)
make test-only                        # tests without re-downloading deps (faster iteration)
make unit-test-with-coverage          # outputs coverage.out
go test -race -run TestName ./pkg/... # run a specific test
```

Test scope: `./pkg/...`, `./cmd/...`, `./internal/...`, `./test/...`

---

## Lint & Format

```bash
make lint                             # golangci-lint (config: .golangci.yml)
make gofmt                            # format with gofmt
make validate                         # check formatting without modifying files
make checklicense                     # verify license headers
make addlicense                       # add missing license headers
make install-tools                    # install golangci-lint, gofumpt, goimports, etc.
```

Limits: 120 char line length, 100 line function length.

---

## Before Submitting a PR

```bash
make test
make lint
make checklicense                     # fix with: make addlicense
make third-party-notices-check        # only if dependencies changed
```

---

## Repository Layout

| Path                    | Purpose                                                                        |
|-------------------------|--------------------------------------------------------------------------------|
| `cmd/`                  | Entry points: `newrelic-infra`, `newrelic-infra-service`, `newrelic-infra-ctl` |
| `pkg/`                  | Public packages — config, backend, metrics, integrations, plugins, log         |
| `internal/`             | Private packages — agent loop, feature flags, httpapi, socketapi               |
| `internal/agent/mocks/` | Generated mocks via mockery — do not edit manually                             |
| `internal/testhelpers/` | Shared test utilities and fixtures                                             |
| `test/`                 | Harvest tests, packaging tests, automated test infrastructure                  |
| `build/`                | Makefiles and scripts for compile, release, CI                                 |
| `tools/`                | Dev utilities: yamlgen, spin-ec2, cdn-purge                                    |
| `vendor/`               | Vendored dependencies (checked in)                                             |

---

## Key Packages

| Package                     | Does                                                                                     |
|-----------------------------|------------------------------------------------------------------------------------------|
| `pkg/config/`               | Config loading; platform-specific defaults; `LoadYamlConfig()` -> config + metadata      |
| `pkg/backend/`              | HTTP transport to NR backend, retry logic, platform-specific clients                     |
| `pkg/integrations/v4/`      | OHI integration protocol, emitter, output parsing                                        |
| `pkg/integrations/legacy/`  | v1/v2 plugin compatibility                                                               |
| `pkg/metrics/`              | CPU, memory, network, storage samplers (platform-specific)                               |
| `internal/agent/`           | Main agent loop, event senders, delta store, command channel                             |
| `internal/integrations/v4/` | Integration executor, definition parsing, file watchers                                  |
| `internal/feature_flags/`   | Feature flag management via command channel                                              |

---

## Code Conventions

**License header** — every file must start with:

```go
// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
```

Run `make addlicense` to add automatically.

**Logger** — use component-tagged logger, never `fmt.Println`:

```go
var clog = log.WithComponent("MyComponent")
clog.WithField("key", value).WithError(err).Error("message")
```

**Error handling** — direct checks, wrap with context:

```go
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
defer func() { _ = resp.Body.Close() }()
```

**Tests** — table-driven with `stretchr/testify`:

```go
cases := []struct {
    name string
    // fields
}{...}
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) {
        assert.Equal(t, expected, actual)
        require.NoError(t, err)
    })
}
```

**Mocks** — generated by `mockery`, stored in `internal/agent/mocks/`. Regenerate, never hand-edit:

```go
ctx := new(mocks.AgentContext)
ctx.On("Config").Return(&config.Config{})
defer ctx.AssertExpectations(t)
```

---

## Platform-Specific Code

Files are suffixed by OS and optionally architecture:

```text
pkg/metrics/cpu_linux.go
pkg/metrics/cpu_darwin.go
pkg/metrics/cpu_windows.go
pkg/config/config_linux_amd64.go
pkg/config/config_darwin_arm64.go
```

Build tags at top of file:

```go
//go:build linux || darwin
```

When touching platform-specific logic, check all variants exist and are consistent.

---

## Integration Development (OHI)

Integrations are external executables producing JSON. Two protocol versions:

- **v4 (current)**: `internal/integrations/v4/`, dimensional metrics
- **v3/legacy**: `pkg/integrations/legacy/`, event-based

Integration config in `newrelic-infra.yml`:

```yaml
integrations:
  - name: nri-flex
    interval: 30s
    timeout: 10s
    env:
      DOCKER_SOCKET: unix:///var/run/docker.sock
```

Key env overrides for container support: `HOST_PROC`, `HOST_SYS`, `HOST_ETC`, `HOST_VAR`.

---

## Environment Variables

| Variable                                           | Purpose                              |
|----------------------------------------------------|--------------------------------------|
| `NRIA_*`                                           | Override any agent config key        |
| `HOST_PROC` / `HOST_SYS` / `HOST_ETC` / `HOST_VAR` | Host root overrides (container runs) |

---

## Generated Code

Do not manually edit:

- `internal/agent/mocks/` — regenerate with `mockery`
- Windows version info — regenerate with `go generate ./cmd/...`

---

## Skills

| Command  | Action                |
|----------|-----------------------|
| `/build` | Compile the agent     |
| `/test`  | Run tests             |
| `/lint`  | Lint and format       |
| `/mock`  | Regenerate mocks      |
| `/pr`    | Create a pull request |

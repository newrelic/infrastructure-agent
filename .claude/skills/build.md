# Build Skill

Build the New Relic infrastructure agent.

## Commands

**Compile for current OS:**

```bash
make compile
```

**Cross-compile for a specific OS:**

```bash
make dist-for-os GOOS=linux
make dist-for-os GOOS=darwin
make dist-for-os GOOS=windows
```

**Build with debug symbols:**

```bash
make debug-for-os GOOS=linux
```

**Create full OS distribution:**

```bash
make dist
```

## Output

Binaries are written to `target/bin/{OS_ARCH}/`:

- `newrelic-infra` — main agent process
- `newrelic-infra-service` — parent service process
- `newrelic-infra-ctl` — troubleshooting utility

## Notes

- Go 1.25+ required
- Use `make compile-centos-5` for CentOS 5 builds (requires Go 1.9)
- Run `make install-tools` first if tools are missing

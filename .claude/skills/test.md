# Test Skill

Run tests for the New Relic infrastructure agent.

## Unit Tests

**Run all unit tests (with race detector):**

```bash
make test
```

**Run tests without re-downloading dependencies:**

```bash
make test-only
```

**Run tests with coverage:**

```bash
make unit-test-with-coverage
# Coverage written to coverage.out
```

**Run a specific test by name:**

```bash
go test -race -run TestFunctionName ./pkg/...
go test -race -run TestFunctionName ./internal/...
go test -race -run TestFunctionName ./cmd/...
```

## Platform-Specific Harvest Tests

```bash
make linux/harvest-tests
make macos/harvest-tests
```

## Integration Tests

**DataBind integration tests:**

```bash
make databind-test
```

**Proxy tests (requires Docker):**

```bash
make proxy-test
```

## Test Scope

Source paths covered by `make test`:

- `./pkg/...`
- `./cmd/...`
- `./internal/...`
- `./test/...`

Timeout: 30 minutes. Flags: `-race -failfast`.

## Notes

- Automated packaging/EC2 tests require `NR_LICENSE_KEY`, `NEW_RELIC_API_KEY`, `NEW_RELIC_ACCOUNT_ID`
- `make install-tools` to install test dependencies if missing

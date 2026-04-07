# Lint Skill

Lint and format the New Relic infrastructure agent codebase.

## Linting

**Run golangci-lint:**

```bash
make lint
```

Config: `.golangci.yml`. Key rules: line length 120, function length 100 lines.

**Validate formatting without changing files:**

```bash
make validate
```

## Formatting

**Format with gofmt:**

```bash
make gofmt
```

Formatters in use: `gofmt`, `gofumpt`, `goimports`.

## License Headers

**Check license headers:**

```bash
make checklicense
```

**Add missing license headers:**

```bash
make addlicense
```

**Validate third-party notices:**

```bash
make third-party-notices-check
```

**Regenerate third-party notices:**

```bash
make third-party-notices
```

## Install Tools

If any linting tools are missing:

```bash
make install-tools
```

Installs: golangci-lint v2.5.0, gofmt, gofumpt, goimports, addlicense, go-licence-detector.

## Notes

- Excluded from linting: `third_party/`, `builtin/`, `examples/`
- Disabled linters: depguard, tagliatelle, testpackage

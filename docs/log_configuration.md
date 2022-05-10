# Log configuration

Previous to Infrastructure Agent version `1.26.0`, the logging configuration was done with any of the following options:

```yaml
# Full path and file name of the log file.
log_file: /tmp/agent.log

# Defines the log output format.
log_format: text

# Set to false to disable logs in the standard output.
log_to_stdout: false

# Enable (1) only for troubleshooting. Use (2) to enable smart verbose. Use (3) to forward the agent logs to 
# New Relic for troubleshooting.

verbose: 0
```

All the previous options are related to logging and in order to unify and simplify its configuration, we have moved them
to a single map configuration variable. With this approach we can remove the `verbose` option and replace it with two
independent options, `level` and `forward`.

Equivalent configuration using the new log configuration:

```yaml
log:
  file: /tmp/agent.log
  format: text
  level: info
  forward: false
  stdout: false
```

The initial configuration options will still be available (at least for two years since the release of `1.26.0`), but we
strongly recommend using the new configuration.
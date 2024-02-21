# Log configuration

Previous to Infrastructure Agent version `1.27.0`, the logging configuration was done with any of the following options:

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

The initial configuration options will still be available (at least for two years since the release of `1.27.0`), but we
strongly recommend using the new configuration.

## Log filters

The new log configuration has added two new options to filter logs based on key-values (fields). They can be used in
order to remove logging noise in a troubleshooting scenario.

By default, all entries will be included* in the logs (`include_filters`) except the integration execution errors. To exclude some entries, we must define the
key-values to remove using the `exclude_filters` option. The following text is a usual agent's log line:

`time="2022-06-10T15:46:38Z" level=debug msg="Integration instances finished their execution. Waiting until next interval." component=integrations.runner.Runner integration_name=nri-flex runner_uid=c03734e49d`

Let's say we want to remove all those lines from the logs, indeed, all the logs referring to the `nri-flex` integration.
The following configuration would solve our needs:

```yaml
log:
  exclude_filters:
    integration_name:
      - nri-flex
```

Note that the previous configuration will remove all the logs related to any `nri-flex` integration. If we want to
exclude a specific `nri-flex` integration configuration, we can use the `runner_id` field:

```yaml
log:
  exclude_filters:
    runner_uid:
      - c03734e49d
```

In an opposite another scenario, we might want to see only the `nri-flex` and `nri-nginx` integrations logs, to do so,
we will need to exclude all the other log entries and specify the corresponding key-values:

```yaml
log:
  exclude_filters:
    "*":
  include_filters:
    integration_name:
      - nri-flex
      - nri-nginx
```

* Only general information from an integration execution is logged, Error and Fatal logs are only visible under debug level. If we want to always show them under any log level for a specific integration we'll need to add it under the include_filters:

```yaml
log:
  include_filters:
    integration_name:
      - nri-flex
      - nri-nginx
```

* The integration supervisor trace logs are excluded by default due the big amount of produced log entries, the
  following configuration would enable them:

```yaml
log:
  exclude_filters:
    "*":
  include_filters:
    traces:
      - supervisor
```

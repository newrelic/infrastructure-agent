# Usage

For the sake of simplicity, this document assumes that the discovery and variables
data binding is working with the Infrastructure Agent integrations.

The configuration has two sections for discovery:

- `variables` is about fetching secret data. It accepts many entries. For each entry,
  only one secret will be retrieved, even if this secret is structured with many fields.
  If the secret is not found, the discovery process fails and return an error.

- `discovery` is about fetching (at the moment) containers data. There is allowed only
  one discovery entry, but it may return multiple matches. The

## Emitted and query-able variables

### Docker & fargate

- `discovery.ip`
- `discovery.port`
- `discovery.image`
- `discovery.name`
- `discovery.label.****`

## Examples

For plugins v4:

```yaml
variables:
  my-variable-name:
    vault:
      http:
         //etc...
discovery:
  ttl: 1h
  docker:
    api_version: 1.39
    match:
      image: mysql
      name: whatever
      label.env: production
      label.newrelic_integration: .
integrations:
  - definition: com.newrelic.mysql // the old integration_name
    name: httpd
    command: all_data
    arguments:
      host: http://"${discovery.ip}":"${discovery.port}"/${discovery.label.status_url | "status"}
  - integration_name: flex
    config:
      - YAML HERE
  - integration_name: ${discovery.label.newrelic_integration}
    config: ${discovery.label.newrelic_config}

```

```yaml
discovery:
  ttl: 1h
  docker:
    api_version: 1.39
    match:
      discovery.label.newrelic_integration: .
integrations:
  - integration_name: ${discovery.label.newrelic_integration}
    config: ${discovery.label.newrelic_config}
```

### Future improvements
* Use default values for unexisting variables
   - E.g. ${discovery.label.url_path | "status_url"}
* query by other fields: networks, etc...
   - optimization: store only values that are queried: verify if it is needed.
* Use optional variables (won't make the process failing if not found)
* Support the following edge case:
```
	// GIVEN a discovery source that returns multiple matches
	// AND a set of variables (secrets) defined by the user
	// WHEN this data is replaced against a template that only refers to a secret
	// (and not discovery data)
	// THEN only one match is returned, since discovery data is not used and only
	// one variable replacement should be done
```
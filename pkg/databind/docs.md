# Usage

For the sake of simplicity, this document assumes that the discovery and variables
data binding is working with the Infrastructure Agent integrations.

The configuration has two sections for discovery:

- [variables](docs/variables.md) is about fetching secret data. It accepts many entries. For each entry,
  only one secret will be retrieved, even if this secret is structured with many fields.
  If the secret is not found, the discovery process fails and return an error.

- [discovery](docs/discovery.md) is about fetching (at the moment) containers data. There is allowed only
  one discovery entry, but it may return multiple matches. 

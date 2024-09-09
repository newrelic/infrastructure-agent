<a href="https://opensource.newrelic.com/oss-category/#community-plus"><picture><source media="(prefers-color-scheme: dark)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/dark/Community_Plus.png"><source media="(prefers-color-scheme: light)" srcset="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Community_Plus.png"><img alt="New Relic Open Source community plus project banner." src="https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Community_Plus.png"></picture></a>

![Build Status](https://github.com/newrelic/infrastructure-agent/actions/workflows/test.yml/badge.svg)
[![Coverage Status](https://coveralls.io/repos/github/newrelic/infrastructure-agent/badge.svg?branch=master)](https://coveralls.io/github/newrelic/infrastructure-agent?branch=master)

# New Relic infrastructure agent

The infrastructure agent (infra-agent) collects inventory data and metrics of your hosts and sends it to the New Relic platform.

[New Relic's infrastructure monitoring](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/get-started/introduction-new-relic-infrastructure) provides flexible,
dynamic monitoring of your entire infrastructure, from services running in the cloud or on dedicated hosts to containers running in orchestrated environments.

- [Compatibility and requirements](#compatibility-and-requirements)
  - [Set up your license key](#set-up-your-license-key)
- [Compile and build the agent](#compile-and-build-the-agent)
- [Run the agent](#run-the-agent)
- [Use the agent](#use-the-agent)
- [Testing](#testing)
- [Documentation](#documentation)
- [To do](#to-do)
- [Support](#support)
- [Privacy](#privacy)
- [Contribute](#contribute)
- [License](#license)

## Compatibility and requirements

You can build the agent for any OS and architecture [supported by Go](https://golang.org/doc/install#requirements);
New Relic does not provide support for all of them.
For more information on operating systems supported by New Relic, see the [agent compatibility docs](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/getting-started/compatibility-requirements-new-relic-infrastructure).

See minimun Go version required on the [go.mod](/go.mod) file.

### Set up your license key

Before using the agent, you need to add the [New Relic license key](https://docs.newrelic.com/docs/accounts/install-new-relic/account-setup/license-key) to the `newrelic-infra.yml` configuration file.
The default location is:

* Linux: `/etc/newrelic-infra.yml`
* Windows: `C:\Program Files\New Relic\newrelic-infra\newrelic-infra.yml`

For more information on configuration methods, precedence, and structure, see the [Configure the Infrastructure agent](https://docs.newrelic.com/docs/infrastructure/install-configure-infrastructure/configuration/configure-infrastructure-agent) document.

## Compile and build the agent

This repository contains a number of scripts that facilitate building `infra-agent` for environments supported by New Relic: Linux, Windows, and Docker.

To build the agent for architectures and OSes different than the one where the build is running, set the [Go environment variables](https://golang.org/cmd/go/#hdr-Environment_variables) to target the desired OS/Architecture combination. For example:

```bash
$ make dist-for-os GOOS=linux
```

To compile and build the agent run these commands:

* Linux:

  ```bash
  $ make compile # On CentOS 5: make compile-centos-5
  $ make dist
  ```
* Windows:
  ```powershell
  PS C:\> ./win_build.ps1
  ```
* Docker: see the [instructions](build/container/README.md) for building a Docker image.

> To build the agent on CentOS 5 use Go 1.9

## Run the agent

Once you've built the agent, you'll find the following binary files inside the `target/bin/{OS_ARCH}` directory:
- `newrelic-infra`: Main process of the agent, tasked with gathering host data and running [integrations](https://docs.newrelic.com/docs/integrations/host-integrations/host-integrations-list).
- `newrelic-infra-service`: Parent process that looks after the main process, making sure it executes and stays up.
- `newrelic-infra-ctl`: Troubleshooting utility that enables verbose logs and executes health checks. For more information, see [Troubleshooting a running agent](https://docs.newrelic.com/docs/infrastructure/install-configure-manage-infrastructure/manage-your-agent/troubleshoot-running-agent).

The agent could run in [root/privileged/unprivileged/administrator(win) mode](https://docs.newrelic.com/docs/infrastructure/install-configure-infrastructure/linux-installation/linux-agent-running-modes).


## Use the agent

You can [start, stop, restart, and check](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/configuration/start-stop-restart-check-infrastructure-agent-status) the Infrastructure agent from the command line.

For more information, [see our official documentation](https://docs.newrelic.com/docs/infrastructure/install-configure-manage-infrastructure).

## Testing

To execute unit tests, run this command:

```bash
$ make test
```

You can run a specific test by invoking `go` (which is also how you can run tests on Windows):
```bash
$ go test -race -run ''      # Run all tests.
$ go test -race -run Foo     # Run top-level tests matching "Foo", such as "TestFooBar".
$ go test -race -run Foo/A=  # For top-level tests matching "Foo", run subtests matching "A=".
$ go test -race -run /A=1    # For all top-level tests, run subtests matching "A=1".
```

For more information, see [Testing](https://golang.org/pkg/testing/) in the official Go docs.

## Automated tests

Packaging and harvest tests can be run in an automated way using ansible.

Provisioning for automated tests is done via terraform:
[Provision](test/provision/README.md)

Packaging and Harvest tests are run using Ansible:
[Packaging tests](test/packaging/ansible/README.md)

[Harvest tests](test/harvest/ansible/README.md)

In order to spin ec2 instances and run these tests automated tests ansible project can be used.

[Automated tests](test/automated/ansible/README.md)

You can use make targets to run spin instances, run tests and terminate instances.

```bash
# spin instances, run tests and terminate instances
make test/automated
```

Individual targets:
```bash
# provision ec2 instances (test/automated/ansible/inventory.ec2 will be created)
make test/provision TAG_OR_UNIQUE_NAME=release_1.0 

# run harvest tests using test/automated/ansible/inventory.ec2 inventory
make test/automated/harvest

# run packaging tests using test/automated/ansible/inventory.ec2 inventory
make test/automated/packaging

# run packaging docker tests using test/automated/ansible/inventory.ec2 inventory
make test/automated/packaging-docker

# terminate instances in test/automated/ansible/inventory.ec2
make test/automated/termination
```

## Documentation

Find more documentation about the overall architecture, components, and workflows in [the docs
directory](docs).

## To do

Provide full CI via GitHub Actions:
- Integrations tests pipeline
  * Platform tests
  * Window tests & builds
  * Installation tests
  * Fuzz testing

## Support

Should you need assistance with New Relic products, you are in good hands with several support diagnostic tools and support channels.

>New Relic offers NRDiag, [a client-side diagnostic utility](https://docs.newrelic.com/docs/using-new-relic/cross-product-functions/troubleshooting/new-relic-diagnostics) that automatically detects common problems with New Relic agents. If NRDiag detects a problem, it suggests troubleshooting steps. NRDiag can also automatically attach troubleshooting data to a New Relic Support ticket. Remove this section if it doesn't apply.

If the issue has been confirmed as a bug or is a feature request, file a GitHub issue.

**Support Channels**

* [New Relic Documentation](https://docs.newrelic.com): Comprehensive guidance for using our platform
* [New Relic Community](https://forum.newrelic.com/: The best place to engage in troubleshooting questions
* [New Relic Developer](https://developer.newrelic.com/): Resources for building a custom observability applications
* [New Relic University](https://learn.newrelic.com/): A range of online training for New Relic users of every level
* [New Relic Technical Support](https://support.newrelic.com/) 24/7/365 ticketed support. Read more about our [Technical Support Offerings](https://docs.newrelic.com/docs/licenses/license-information/general-usage-licenses/support-plan).

## Privacy

At New Relic we take your privacy and the security of your information seriously, and are committed to protecting your information. We must emphasize the importance of not sharing personal data in public forums, and ask all users to scrub logs and diagnostic information for sensitive information, whether personal, proprietary, or otherwise.

We define “Personal Data” as any information relating to an identified or identifiable individual, including, for example, your name, phone number, post code or zip code, Device ID, IP address, and email address.

For more information, review [New Relic’s General Data Privacy Notice](https://newrelic.com/termsandconditions/privacy).

## Contribute

We encourage your contributions to improve this project! Keep in mind that when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.

If you have any questions, or to execute our corporate CLA (which is required if your contribution is on behalf of a company), drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

If you would like to contribute to this project, review [these guidelines](./CONTRIBUTING.md).

To all contributors, we thank you!  Without your contribution, this project would not be what it is today.

## License

infrastructure-agent is licensed under the [Apache 2.0](https://apache.org/licenses/LICENSE-2.0.txt) License.


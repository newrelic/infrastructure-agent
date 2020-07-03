[![Community Project header](https://github.com/newrelic/open-source-office/raw/master/examples/categories/images/Community_Project.png)](https://github.com/newrelic/open-source-office/blob/master/examples/categories/index.md#community-project)

# New Relic infrastructure agent

The infrastructure agent (infra-agent) collects inventory data and metrics of your hosts and sends it to the New Relic platform. 

[New Relic](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/get-started/introduction-new-relic-infrastructure) provides flexible,
dynamic monitoring of your entire infrastructure,
from services running in the cloud or on dedicated hosts to containers running in orchestrated environments.

* [Compatibility and requirements](#compatibility-and-requirements)
* [Compile and build the agent](#compile-and-build-the-agent)
* [Run the agent](#run-the-agent)
* [Use the agent](#use-the-agent)
* [Testing](#testing)
* [Support](#support)
* [Contributing](#contributing)
* [To-do](#to-do)
* [License](#license)

## Compatibility and requirements

Go 1.11 or higher is required to build the infrastructure agent.

You can build the agent for any OS and architecture [supported by Go](https://golang.org/doc/install#requirements);
New Relic does not provide support for all of them.
For more information on operating systems supported by New Relic, see the [agent compatibility docs](https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/getting-started/compatibility-requirements-new-relic-infrastructure).

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
* Docker: see the [instructions](/build/container/README.md) for building a Docker image.

> To build the agent on CentOS 5 use Go 1.9

## Run the agent

Once you've built the agent, you'll find the following binary files inside the `target/bin/{OS_ARCH}` directory:
- `newrelic-infra`: Main process of the agent, tasked with gathering host data and running [integrations](https://docs.newrelic.com/docs/integrations/host-integrations/host-integrations-list).
- `newrelic-infra-service`: Parent process that looks after the main process, making sure it executes and stays up.
- `newrelic-infra-ctl`: Troubleshooting utility that enables verbose logs and executes health checks. For more information, see [Troubleshooting a running agent](https://docs.newrelic.com/docs/infrastructure/install-configure-manage-infrastructure/manage-your-agent/troubleshoot-running-agent).

The agent must run in [root/administrator mode](https://docs.newrelic.com/docs/infrastructure/install-configure-infrastructure/linux-installation/linux-agent-running-modes).


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

## Support

New Relic provides support for its infrastructure agent when it's installed using the [official packages](https://docs.newrelic.com/docs/infrastructure/install-configure-manage-infrastructure).
You can build the agent from the source and use it with New Relic without official support:
To get support for features or systems you've added, we strongly encourage you to [open a pull request](CONTRIBUTING.md).

For more information about reporting security issues, see our [reporting security process](https://docs.newrelic.com/docs/security/new-relic-security/data-privacy/reporting-security-vulnerabilities)

### New Relic Explorers Hub

New Relic hosts and moderates an online forum where customers can interact with New Relic employees as well as other customers to get help and share best practices.
Like all official New Relic open source projects, there's a related Community topic in the New Relic Explorers Hub.
You can find this project's topic/threads here:

https://discuss.newrelic.com/c/support-products-agents/new-relic-infrastructure

## Contributing

Contributions to improve infrastructure agent are encouraged!
Keep in mind when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant.
You only have to sign the CLA one time per project.

To execute our corporate CLA, which is required if your contribution is on behalf of a company, or if you have any questions, please drop us an email at open-source@newrelic.com.

Full details about how to contribute in our [Contribution Guide](./CONTRIBUTING.md).

## To do

Provide full CI via GitHub Actions:
- Integrations tests pipeline
  * Platform tests
  * Window tests & builds
  * Installation tests
  * Fuzz testing
- Release pipeline

## Code of Conduct

Read our [Code of Conduct](./CODE_OF_CONDUCT.md) to understand how we operate in open source and what we expect of contributors.
  
## License

Infrastructure Agent is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.

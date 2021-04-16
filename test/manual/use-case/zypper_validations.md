### Scope
This document share which are the commands and validations needed to passed all acceptance criteria for the manual system acceptance test.
It covers support for use case of:
* SLES 12.04

### Setup
- For [SLES 12.04](https://app.vagrantup.com/wandisco/boxes/sles-12.4-64)
```shell script
$ sudo curl -o /etc/zypp/repos.d/newrelic-infra.repo http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/zypp/sles/12.4/x86_64/newrelic-infra.repo
```

### Test suit
#### Scenario 1. Package naming follow dist convention
Lookup at apt repository folder and compare package under test with the previous package version name. \
e.g.: [ZYPP Repository folder](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/zypp/sles/12.4/x86_64/) \
package under test version: 1.16.1, name: newrelic-infra-1.16.1-1.sles12.4.x86_64.rpm \
previous package version: 1.16.0, name: newrelic-infra-1.16.0-1.sles12.4.x86_64.rpm

#### Scenario 2. Package contains all required files
Review file managed by package and compare with previous package version.
```shell script
$ sudo rpm -ql newrelic-infra
```
e.g: expected output:
```shell script
/etc/newrelic-infra/integrations.d/docker-config.yml
/etc/newrelic-infra/logging.d/file.yml.example
/etc/newrelic-infra/logging.d/fluentbit.yml.example
/etc/newrelic-infra/logging.d/syslog.yml.example
/etc/newrelic-infra/logging.d/systemd.yml.example
/etc/newrelic-infra/logging.d/tcp.yml.example
/etc/systemd/system/newrelic-infra.service
/usr/bin/newrelic-infra
/usr/bin/newrelic-infra-ctl
/usr/bin/newrelic-infra-service
/usr/lib/.build-id
/usr/lib/.build-id/5c
/usr/lib/.build-id/5c/c36c6cb0850c532449ab2940aba99a9ae4dacf
/var/db/newrelic-infra/LICENSE.txt
/var/db/newrelic-infra/custom-integrations
/var/db/newrelic-infra/integrations.d
/var/db/newrelic-infra/newrelic-integrations/bin/nri-docker
/var/db/newrelic-infra/newrelic-integrations/bin/nri-flex
/var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus
/var/db/newrelic-infra/newrelic-integrations/logging/fluent-bit
/var/db/newrelic-infra/newrelic-integrations/logging/out_newrelic.so
/var/db/newrelic-infra/newrelic-integrations/logging/parsers.conf
/var/log/newrelic-infra
/var/run/newrelic-infra
```

#### Scenario 3. Check agent version
Check if version number is well inform.
```shell script
$ newrelic-infra -version
```
expected output:
```shell script
> New Relic Infrastructure Agent version: 1.16.1, GoVersion: go1.14.4, GitCommit: ...
```

#### Scenario 4. Service is working
Check if agent is running and sending metrics to NR.
```shell script
$ sudo systemctl show newrelic-infra --no-page|grep SubState=running
```
expected output: 
```shell script
> SubState=running
```

Platform validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT count(*) from SystemSample where displayName = '${DISPLAY_NAME}'"
```
e.g. expected output: 
```json
[
  {
    "agentName": "Infrastructure",
    "agentVersion": "1.16.1",
    "displayName": "test-script",
    "linuxDistribution": "SUSE Linux Enterprise Server 12 SP2"
  }
]
```

#### Scenario 5. Package metadata is valid
Review if basic metadata is in place.
```shell script
$ rpm -qi newrelic-infra
```
e.g: expected output:
```shell script
Name        : newrelic-infra
Epoch       : 0
Version     : 1.16.1
Release     : 1.sles12.4
Architecture: x86_64
Install Date: Tue 09 Mar 2021 04:46:17 PM UTC
Group       : default
Size        : 135872380
License     : Copyright (c) 2008-2021 New Relic, Inc. All rights reserved.
Signature   : RSA/SHA1, Wed 17 Feb 2021 02:55:10 PM UTC, Key ID bb29ee038ecce87c
Source RPM  : newrelic-infra-1.15.2-1.sles12.4.src.rpm
Build Date  : Wed 10 Feb 2021 10:13:03 AM UTC
Build Host  : 7c9aed865529
Relocations : /
Packager    : caos-team@newrelic.com
Vendor      : New Relic, Inc.
URL         : https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes/new-relic-infrastructure-agent-1152
Summary     : New Relic Infrastructure Agent
Description :
New Relic Infrastructure provides flexible, dynamic server monitoring. With real-time data collection and a UI that scales from a handful of hosts to thousands, Infrastructure is designed for modern Operations teams with fast-changing systems.
Distribution: (none)
```

#### Scenario 6. Package signature is valid
Review if pub GPG key is same as PROD.
```shell script
$ gpg --keyid-format short --list-keys
```
e.g.: expected output:
```shell script
pub   rsa4096 2016-10-26 [SCEA] <<GUID_GPG_KEY>>
uid           [ unknown] infrastructure-eng <infrastructure-eng@newrelic.com>
```

#### Scenario 7. Agent privileged mode is working
For this use case You should install the agent with privileged mode.
```shell script
$ sudo NRIA_MODE=PRIVILEGED zypper -n install newrelic-infra
```

Platform validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```

#### Scenario 8. Agent unprivileged mode is working
Similar to previous scenario You should install the agent with unprivileged mode.
```shell script
$ sudo NRIA_MODE=UNPRIVILEGED yum install newrelic-infra -y
```

Platform validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```

#### Scenario 9. Package uninstall
```shell script
$ sudo zypper remove newrelic-infra
```
Platform Validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```
no data should be returned.

#### Scenario 10. Package upgrade
With an old agent version install, install the latest.
```shell script
$ sudo zypper -n install newrelic-infra-1.15.1
$ newrelic-infra -version
$ sudo zypper -n install newrelic-infra
$ newrelic-infra -version
```
Platform Validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```

#### Scenario 11. Built in Flex integration is working
Add Flex example yml file and review data in NR.
```shell script
$ sudo curl -o /etc/newrelic-infra/integrations.d/flex-dig.yml https://raw.githubusercontent.com/newrelic/nri-flex/master/examples/linux/dig-example.yml

$ sudo systemctl restart newrelic-infra
```

Platform verification:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT uniques(integrationVersion) from flexStatusSample where displayName = '${DISPLAY_NAME}'"
```
e.g: expected output:
```json
[
  {
    "uniques.integrationVersion": [
      "1.4.0"
    ]
  }
]
```

#### Scenario 11. Built in Log-forwarded integration is working
Enable verbose mode in agent configuration file and review data in NR.
```shell script
$ sudo sed -i 's#verbose:.*#verbose: 3#g' /etc/newrelic-infra.yml
$ sudo systemctl restart newrelic-infra
```
Platform verification:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT count(*) from Log where displayName = '${DISPLAY_NAME}'"
```
e.g: expected output:
```json
[
    {
      "count": 31
    }
]
```

#### Scenario 12. Built in Prometheus integration is working
Check if binary works.
```shell script
$ /var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus --help
```
expected value:
```shell script
Usage of /var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus:
  -config_path string
    	Path to the config file
  -configfile string
    	Deprecated. --config_path takes precedence if both are set
```
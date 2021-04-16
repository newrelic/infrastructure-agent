### Scope
This document share which are the commands and validations needed to passed all acceptance criteria for the manual system acceptance test.
It covers support for use case of:
* Red Hat Enterprise Linux version 8, 7 and 6
* CentOs version 8, 7, 6

### Setup
- For [RHEL 6](https://app.vagrantup.com/generic/boxes/rhel6) or [CentOs 6](https://app.vagrantup.com/generic/boxes/centos6):
    ```shell script
    $ sudo curl -o /etc/yum.repos.d/newrelic-infra.repo http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/yum/el/6/x86_64/newrelic-infra.repo
    ```
- For [RHEL 7](https://app.vagrantup.com/generic/boxes/rhel7) or [CentOs 7](https://app.vagrantup.com/generic/boxes/centos7):
    ```shell script
    $ sudo curl -o /etc/yum.repos.d/newrelic-infra.repo http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/yum/el/7/x86_64/newrelic-infra.repo
    ```
- For [RHEL 8](https://app.vagrantup.com/generic/boxes/rhel8) or [CentOs 8](https://app.vagrantup.com/generic/boxes/centos8):
    ```shell script
    $ sudo curl -o /etc/yum.repos.d/newrelic-infra.repo http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/yum/el/8/x86_64/newrelic-infra.repo
    ```

### Test suit
#### Scenario 1. Package naming follow dist convention
Lookup at apt repository folder and compare package under test with the previous package version name. \
e.g.: [YUM v8 Repository folder](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/yum/el/8/x86_64/) \
package under test version: 1.16.1, name: newrelic-infra-1.16.1-1.el8.x86_64.rpm \
previous package version: 1.16.0, name: newrelic-infra-1.16.0-1.el8.x86_64.rpm

#### Scenario 2. Package contains all required files
Review file managed by package and compare with previous package version.
```shell script
$ repoquery -l newrelic-infra --installed
```
e.g: RHEL 6 output:
```shell script
/etc/init.d/newrelic-infra
/etc/init/newrelic-infra.conf
/etc/newrelic-infra/integrations.d/docker-config.yml
/usr/bin/newrelic-infra
/usr/bin/newrelic-infra-ctl
/usr/bin/newrelic-infra-service
/var/db/newrelic-infra/LICENSE.txt
/var/db/newrelic-infra/custom-integrations
/var/db/newrelic-infra/integrations.d
/var/db/newrelic-infra/newrelic-integrations/bin/nri-docker
/var/db/newrelic-infra/newrelic-integrations/bin/nri-flex
/var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus
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
    "displayName": "centos7-test",
    "linuxDistribution": "CentOS Linux 7 (Core)"
  }
]
```
<sub><sup>note: CentOS6 and RHEL6 run `$ sudo initctl status newrelic-infra` instate.</sub></sup>

#### Scenario 5. Package metadata is valid
Review if basic metadata is in place.
```shell script
$ sudo yum info newrelic-infra -q
```
e.g: expected output:
```shell script
Installed Packages
Name        : newrelic-infra
Arch        : x86_64
Version     : 1.16.11
Release     : 1.el7
Size        : 112 M
Repo        : installed
From repo   : newrelic-infra
Summary     : New Relic Infrastructure Agent
URL         : https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes
License     : Copyright (c) 2008-2021 New Relic, Inc. All rights reserved.
Description : New Relic Infrastructure provides flexible, dynamic server monitoring. With real-time data collection and
            : a UI that scales from a handful of hosts to thousands, Infrastructure is designed for modern Operations
            : teams with fast-changing systems.
```

#### Scenario 6. Package signature is valid
Adding a repository in YUM is a manual operation, which consists in creating a file with the .repo extension under the folder /etc/yum.repos.d.
The file must contain all the information about the custom repository that we are connecting to, therefore in setup step we already downloading the GPG.
What You can check if it was correctly added to rpm:
```shell script
$ rpm -qa gpg-pubkey --qf "%{summary}\n" | grep infrastructure-eng
```
expected output:
```shell script
gpg(infrastructure-eng <infrastructure-eng@newrelic.com>)
```

#### Scenario 7. Agent privileged mode is working
For this use case You should install the agent with privileged mode.
```shell script
$ sudo NRIA_MODE=PRIVILEGED yum install newrelic-infra -y
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
$ sudo yum remove newrelic-infra -y
```
Platform Validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```
no data should be returned.

#### Scenario 10. Package upgrade
With an old agent version install, install the latest.
```shell script
$ sudo yum install newrelic-infra=1.15.1 -y --allow-downgrades
$ newrelic-infra -version
$ sudo yum install newrelic-infra -y
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

$ sudo systemctl newrelic-infra restart
```
<sub><sup>notes: for version 6 use `sudo initctl newrelic-infra restart`</sub></sup>

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
<sub><sup>notes: not supported in version 6.</sub></sup>

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

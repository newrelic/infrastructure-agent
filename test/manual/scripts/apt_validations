### Scope
This document share which are the commands and validations needed to passed all acceptance criteria for the manual system acceptance test.
It covers support for use case of:
* Ubuntu 20, 18, 16 and 14
* Debian 10

### Setup
- For [Ubuntu 20.04](https://app.vagrantup.com/generic/boxes/ubuntu2004) (focal):
    ```
    $ printf "deb [arch=amd64] http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/test/linux/apt focal main" | sudo tee -a /etc/apt/sources.list.d/newrelic-infra.list
    ```
- For [Ubuntu 18.04](https://app.vagrantup.com/generic/boxes/ubuntu1804) (bionic):
    ```
    $ printf "deb [arch=amd64] http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/test/linux/apt bionic main" | sudo tee -a /etc/apt/sources.list.d/newrelic-infra.list
    ```
- For Ubuntu [16.04](https://app.vagrantup.com/generic/boxes/ubuntu1604) (xenial):
    ```
    $ printf "deb [arch=amd64] http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/test/linux/apt xenial main" | sudo tee -a /etc/apt/sources.list.d/newrelic-infra.list   
    ```   
- For Ubuntu [14.04](https://app.vagrantup.com/ubuntu/boxes/trusty64) (trusty):
    ```
    $ printf "deb [arch=amd64] http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/test/linux/apt trusty main" | sudo tee -a /etc/apt/sources.list.d/newrelic-infra.list
    ```  
- For [Debian 10](https://app.vagrantup.com/generic/boxes/debian10) (buster):
    ```
    $ printf "deb [arch=amd64] http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/apt buster main" | sudo tee -a /etc/apt/sources.list.d/newrelic-infra.list
    ```  

### Test suit
#### Scenario 1. Package naming follow dist convention
Lookup at apt repository folder and compare package under test with the previous package version name. \
e.g.: [APT Repository folder](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/apt/pool/main/n/newrelic-infra/) \
package under test version: 1.16.1, name: newrelic-infra_systemd_1.16.1_amd64.deb \
previous package version: 1.16.0, name: newrelic-infra_systemd_1.16.0_amd64.deb

<sub><sup>note: Ubuntu 14 use upstart service manager therefore package name will be newrelic-infra_upstart_1.16.0_amd64.deb</sub></sup>

#### Scenario 2. Package contains all required files
Review file managed by package and compare with previous package version.
```
$ sudo dpkg -L newrelic-infra
```
e.g: Debian 10 output:
```
/var/db/newrelic-infra
/var/db/newrelic-infra/custom-integrations
/var/db/newrelic-infra/integrations.d
/var/log/newrelic-infra
/var/run/newrelic-infra
/usr/bin/newrelic-infra-ctl
/usr/bin/newrelic-infra
/usr/bin/newrelic-infra-service
/var/db/newrelic-infra/LICENSE.txt
/etc/newrelic-infra
/etc/newrelic-infra/logging.d
/etc/newrelic-infra/logging.d/file.yml.example
/etc/newrelic-infra/logging.d/fluentbit.yml.example
/etc/newrelic-infra/logging.d/syslog.yml.example
/etc/newrelic-infra/logging.d/systemd.yml.example
/etc/newrelic-infra/logging.d/tcp.yml.example
/var/db/newrelic-infra/newrelic-integrations
/var/db/newrelic-infra/newrelic-integrations/logging
/var/db/newrelic-infra/newrelic-integrations/logging/parsers.conf
/etc/systemd/system/newrelic-infra.service
/var/db/newrelic-infra/newrelic-integrations/logging/out_newrelic.so
/var/db/newrelic-infra/newrelic-integrations/logging/fluent-bit
/etc/newrelic-infra/integrations.d
/etc/newrelic-infra/integrations.d/docker-config.yml
/var/db/newrelic-infra/newrelic-integrations/bin
/var/db/newrelic-infra/newrelic-integrations/bin/nri-docker
/var/db/newrelic-infra/newrelic-integrations/bin/nri-flex
/var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus
```

#### Scenario 3. Check agent version
Check if version number is well inform.
```
$ newrelic-infra -version
```
expected output:
```
> New Relic Infrastructure Agent version: 1.16.1, GoVersion: go1.14.4, GitCommit: ...
```

#### Scenario 4. Service is working
Check if agent is running and sending metrics to NR.
```
$ sudo systemctl show newrelic-infra --no-page|grep SubState=running
```
expected output: 
```

```

Platform validation:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT count(*) from SystemSample where displayName = '${DISPLAY_NAME}'"
```
e.g. expected output: 
```
[
  {
    ...
    "agentName": "Infrastructure",
    "agentVersion": "1.16.0",
    ...
    "displayName": "deb10-test",
    ...
    "linuxDistribution": "Debian GNU/Linux 10 (buster)",
    ...
  }
]
```

#### Scenario 5. Package metadata is valid
Review if basic metadata is in place.
```
$ apt show newrelic-infra
```
e.g: expected output:
```
Package: newrelic-infra
Version: 1.16.1
Priority: extra
Section: default
Maintainer: caos-team@newrelic.com
Installed-Size: 118 MB
Conflicts: newrelic-infra
Replaces: opspro-agent, opspro-agent-sysv
Homepage: https://docs.newrelic.com/docs/release-notes/infrastructure-release-notes/infrastructure-agent-release-notes
Vendor: New Relic, Inc.
Download-Size: 40.6 MB
APT-Manual-Installed: yes
APT-Sources: http://nr-downloads-ohai-testing.s3-website-us-east-1.amazonaws.com/infrastructure_agent/linux/apt focal/main amd64 Packages
Description: New Relic Infrastructure provides flexible, dynamic server monitoring. With real-time data collection and a UI that scales from a handful of hosts to thousands, Infrastructure is designed for modern Operations teams with fast-changing systems.
```
<sub><sup>note: for Debian 10 You can use `aptitude show`</sub></sup>

#### Scenario 6. Package signature is valid
Review if pub GPG key is same as PROD.
```
$ apt-key list | grep newrelic -n2
```
e.g.: expected output:
```
pub   rsa4096 2016-10-26 [SCEA] <<GUID_GPG_KEY>>
uid           [ unknown] infrastructure-eng <infrastructure-eng@newrelic.com>
/etc/apt/trusted.gpg.d/ubuntu-keyring-2012-archive.gpg
```

#### Scenario 7. Agent privileged mode is working
For this use case You should install the agent with privileged mode.
```
$ sudo NRIA_MODE=PRIVILEGED apt install newrelic-infra -y
```
Platform validation:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```

#### Scenario 8. Agent unprivileged mode is working
Similar to previous scenario You should install the agent with unprivileged mode.
```
$ sudo NRIA_MODE=UNPRIVILEGED apt install newrelic-infra -y
```
Platform validation:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```

#### Scenario 9. Package uninstall
```
$ sudo apt remove newrelic-infra -y 
```
Platform Validation:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```
no data should be returned.

#### Scenario 10. Package upgrade
With an old agent version install, install the latest.
```
$ sudo apt install newrelic-infra=1.15.1 -y --allow-downgrades
$ newrelic-infra -version
$ sudo apt install newrelic-infra -y
$ newrelic-infra -version
```

#### Scenario 11. Built in Flex integration is working
Add Flex example yml file and review data in NR.
```
$ sudo curl -o /etc/newrelic-infra/integrations.d/flex-dig.yml https://raw.githubusercontent.com/newrelic/nri-flex/master/examples/linux/dig-example.yml

$ sudo service newrelic-infra restart
```
Platform verification:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT uniques(integrationVersion) from flexStatusSample where displayName = '${DISPLAY_NAME}'"
```
e.g: expected output:
```
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
```
$ sudo sed -i 's#verbose:.*#verbose: 3#g' /etc/newrelic-infra.yml
$ sudo systemctl restart newrelic-infra
```
Platform verification:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT count(*) from Log where displayName = '${DISPLAY_NAME}'"
```
e.g: expected output:
```
[
    {
      "count": 31
    }
]
```

#### Scenario 12. Built in Prometheus integration is working
Check if binary works.
```
$ /var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus --help
```
expected value:
```
Usage of /var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus:
  -config_path string
    	Path to the config file
  -configfile string
    	Deprecated. --config_path takes precedence if both are set
```

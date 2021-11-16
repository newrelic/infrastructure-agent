### Scope
This document share which are the commands and validations needed to passed all acceptance criteria for the manual system acceptance test.
It covers support for use case of:
* Docker amd64, arm64

### Setup
  - For [amd64](https://app.vagrantup.com/bento/boxes/ubuntu-20.04)
  - For [arm64](https://aws.amazon.com/ec2/graviton/)

Steps to install containerize agent [here](https://hub.docker.com/r/newrelic/infrastructure/) or using docker-compose:
```shell script
version: '3'
services:
  agent-rc:
    image: newrelic/infrastructure:1.16.1-rc
    container_name: infra-agent-rc
    cap_add:
      - SYS_PTRACE
    network_mode: host
    pid: host
    privileged: true
    environment:
      - NRIA_LICENSE_KEY=${NR_LICENSE_KEY}
      - NRIA_STAGING=true
      - NRIA_DISPLAY_NAME=${DISPLAY_NAME}
    volumes:
      - "/:/host:ro"
      - "/var/run/docker.sock:/var/run/docker.sock"
    restart: unless-stopped
```
Then up the container:
```shell script
$ sudo -E docker-compose up
```

### Test suit
#### Scenario 1. Package naming follow dist convention
Lookup at docker hub and compare tag under test with the previous tag name. \
e.g.: [Docker hub](https://hub.docker.com/r/newrelic/infrastructure/tags?page=1&ordering=last_updated) \
Check arch:
```shell script
$ uname -a
```
expected output:
```shell script
> Linux ip-172-31-19-77.eu-central-1.compute.internal 4.14.219-161.340.amzn2.aarch64 #1 SMP Thu Feb 4 05:54:27 UTC 2021 aarch64 aarch64 aarch64 GNU/Linux
```

#### Scenario 2. Package contains all required files
Review file managed by package and compare with previous package version.
```shell script
$ du -a /var/db/newrelic-infra/
```
e.g: arm64 output:
```shell script
20160	/var/db/newrelic-infra/newrelic-integrations/bin/nri-flex
24832	/var/db/newrelic-infra/newrelic-integrations/bin/nri-prometheus
44992	/var/db/newrelic-infra/newrelic-integrations/bin
44992	/var/db/newrelic-infra/newrelic-integrations
```

#### Scenario 3. Check agent version
Check if version number is well inform.
```shell script
$ sudo docker exec -it <<CONTAINER_ID>> /bin/bash -c 'newrelic-infra -version'
```
expected output:
```shell script
> New Relic Infrastructure Agent version: 1.16.1, GoVersion: go1.16.9, GitCommit: ...
```

#### Scenario 4. Service is working
Check if agent is sending metrics to NR.
Platform validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT count(*) from SystemSample where displayName = '${DISPLAY_NAME}'"
```
e.g. expected output: 
```json
[
  {
    "agentName": "Infrastructure",
    "agentVersion": "1.16.0",
    "displayName": "docker-test",
    "linuxDistribution": "Ubuntu Core 18"
  }
]
```

#### Scenario 5. Package metadata is valid
NA

#### Scenario 6. Package signature is valid
NA

#### Scenario 7. Agent privileged mode is working
For this use case You should enable in docker-compose file the privileged mode.
```yaml
    privileged: true
```
Platform validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```

#### Scenario 8. Agent unprivileged mode is working
NA

#### Scenario 9. Package uninstall
```shell script
$ sudo docker-compose down
```
Platform Validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```
no data should be returned.

#### Scenario 10. Package upgrade
NA

#### Scenario 11. Built in Flex integration is working
Add Flex example yml file and review data in NR.
e.g.: Dockerfile:
```Dockerfile
FROM newrelic/infrastructure:1.16.1-rc-arm64
ADD newrelic-infra.yml /etc/newrelic-infra.yml
RUN mkdir /etc/newrelic-infra
RUN mkdir /etc/newrelic-infra/integrations.d/
RUN wget https://raw.githubusercontent.com/newrelic/nri-flex/master/examples/windows/windows-uptime.yml -P /etc/newrelic-infra/integrations.d/
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
Not supported yet.

#### Scenario 12. Built in Prometheus integration is working
Not supported yet.

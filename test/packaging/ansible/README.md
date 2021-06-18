## Automated packaging tests

The goal of this ansible project is to run infra-agent installation tests.
Tests will be executed in hosts under `testing_hosts` group in ansible inventory.

```
[localhost]
localhost ansible_connection=local

[testing_hosts]
amd64:debian-buster ansible_host=192.168.1.12 ansible_user=admin ansible_python_interpreter=/usr/bin/python3 
amd64:centos7 ansible_host=192.168.1.13 ansible_user=centos ansible_python_interpreter=/usr/bin/python 
```

## Playbooks

* [installation-pinned.yml](installation-pinned.yml): Install specific version of the infra-agent. 
  Agent version is specified by `target_agent_version` variable.
  

* [installation-privileged.yml](installation-privileged.yml): Install latest version of the infra-agent in `PRIVILEGED` mode.
  

* [installation-unprivileged.yml](installation-unprivileged.yml): Install latest version of the infra-agent in `UNPRIVILEGED` mode.
    

* [installation-root.yml](installation-root.yml): Install latest version of the infra-agent in `ROOT` mode.
  

* [agent-upgrade.yml](agent-upgrade.yml): Install pinned version of the infra-agent and upgrade to latest.
  

* [log-forwarder.yml](log-forwarder.yml): Test log forwarder is running as expected.


* [test.yml](test.yml): Run all playbooks.


  
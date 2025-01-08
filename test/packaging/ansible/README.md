## Automated packaging tests

The goal of this ansible project is to run infra-agent installation tests.
Tests will be executed in hosts under `testing_hosts` group in ansible inventory.

```
[localhost]
localhost ansible_connection=local

[testing_hosts]
amd64:debian-buster ansible_host=192.168.1.12 ansible_user=admin ansible_python_interpreter=/usr/bin/python3 
amd64:centos7 ansible_host=192.168.1.13 ansible_user=centos ansible_python_interpreter=/usr/bin/python
amd64:al-2023-fips ansible_host=192.168.1.14 ansible_user=ec2-user ansible_python_interpreter=/usr/bin/python3 ansible_ssh_common_args='-o Ciphers=aes256-ctr,aes192-ctr,aes128-ctr -o KexAlgorithms=ecdh-sha2-nistp256,ecdh-sha2-nistp384,ecdh-sha2-nistp521 -o MACs=hmac-sha2-256,hmac-sha2-512'
```

## Playbooks

* [installation-pinned.yml](installation-pinned.yml): Install specific version of the infra-agent. 
  Agent version is specified by `target_agent_version` variable.
  

* [installation-privileged.yml](installation-privileged.yml): Install latest version of the infra-agent in `PRIVILEGED` mode.
  

* [installation-unprivileged.yml](installation-unprivileged.yml): Install latest version of the infra-agent in `UNPRIVILEGED` mode.
    

* [installation-root.yml](installation-root.yml): Install latest version of the infra-agent in `ROOT` mode.
  

* [agent-upgrade.yml](agent-upgrade.yml): Install pinned version of the infra-agent and upgrade to latest.
  

* [log-forwarder.yml](log-forwarder.yml): Test log forwarder is running as expected.


* [shutdown-and-terminate.yml](shutdown-and-terminate.yml): Test infra agent graceful shutdown and HNR alert.


* [test.yml](test.yml): Run all playbooks.


  
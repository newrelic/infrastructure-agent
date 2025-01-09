## Automated harvest tests

The goal of this ansible project is to build harvest tests in the host machine and copy and run then in the provided instances. 
With this approach we can automatise these tests and ensure that tests are run in all supported architectures/distributions.
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

* [test.yml](test.yml): Build harvest test for provided architectures/os combinations, copy binaries to
  provisioned instances and run them.
  
  [Default os/arch combinations](roles/build-harvest-tests/vars/main.yml):
```yaml
goos:
  - "linux"
goarch:
  - "amd64"
  - "arm"
  - "arm64"
```

### Run specific tests:

Use `tests_to_run_regex` to run a subset of tests based on provided regular expression

Examples:

```bash 
# run tests that match regex ^TestHeartBeatSampler$
ansible-playbook -i inventory.ini -e tests_to_run_regex="^TestHeartBeatSampler$" test.yml
```

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

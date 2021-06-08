## Automated harvest tests

The goal of this ansible project is to provision several ec2 instances, run harvest tests in these instances and terminate them. 
With this approach we can automatise these tests and ensure that tests are run in all supported architectures/distributions.
All the ec2 instances created will have `instance_name_tag_prefix` as prefix in the instance name, so they can be located
for termination.

## Playbooks

The test is divided in 3 playbooks:

* [provision-ec2.yml](provision-ec2.yml): Spin ec2 instances based on definition provided in `instances` variable.
  Default values can be found in [group_vars/localhost/main.yml](group_vars/localhost/main.yml). 
  ```yaml
  # Instances definition
  instances:
  - ami: "ami-03b6c8bd55e00d5ed"
    type: "t3a.small"
    name: "amd64:ubuntu20.04"
    username: "ubuntu"
    python_interpreter: "/usr/bin/python3"
  - ami: "ami-0600b1bef20a0c212"
    type: "t3a.small"
    name: "amd64:ubuntu18.04"
    username: "ubuntu"
    python_interpreter: "/usr/bin/python3"
  # Instance with custom launch_template
  - ami: "ami-0d6d3e4f081c69f42"
    type: "t3a.small"
    name: "amd64:debian-stretch"
    username: "admin"
    python_interpreter: "/usr/bin/python3"
    launch_template: "LaunchTemplateId=lt-01b2c565029b5bf2a,Version=1"
  ```
  All spun instances will be named with the specified `name` variable in `instances` prefixed by the prefix `instance_name_tag_prefix`
  defined in [group_vars/localhost/main.yml](group_vars/localhost/main.yml). This prefix will be used to terminate the 
  spun instances.


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


* [terminate-ec2.yml](terminate-ec2.yml): Terminate instances. `instance_name_tag_prefix` Will be used to retrieve the 
  instances to be terminated.
  
## Execution
Ensure `AWS_PROFILE` and `AWS_REGION` env variables are exported before running the test.
```bash 
# from the agent root folder
make run-automated-harvest-tests
```

`PROVISION` and `TERMINATE`environment variable can be used to run tests without provisioning/terminating instances.
```bash 
# from the agent root folder
TERMINATE=0 make run-automated-harvest-tests
TERMINATE=0 PROVISION=0 make run-automated-harvest-tests
```

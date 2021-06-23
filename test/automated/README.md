## Automated tests

The goal of this ansible project is to provision several ec2 instances and create an ansible inventory file with the
instances created. When these instances are not needed longer they can be terminated using the same inventory.

[install-python.yml](install-python.yml) can be used to install python into provisioned instances that do not have python installed by default.

## Playbooks

* [provision.yml](provision.yml): Spin ec2 instances based on definition provided in `instances` variable.
  Default values can be found in [group_vars/localhost/main.yml](group_vars/localhost/main.yml). 
  ```yaml
  # Prefix to be used for the provisioned instances names
  provision_host_prefix: "harvest-tests"
  
  # Instances definition
  instances:
  - ami: "ami-03b6c8bd55e00d5ed"
    type: "t3a.small"
    name: "amd64:ubuntu20.04"
    username: "ubuntu"
    python_interpreter: "/usr/bin/python3"
    launch_template: "LaunchTemplateId=lt-01b2c565029b5bf2a,Version=1"
  - ami: "ami-0600b1bef20a0c212"
    type: "t3a.small"
    name: "amd64:ubuntu18.04"
    username: "ubuntu"
    python_interpreter: "/usr/bin/python3"
    launch_template: "LaunchTemplateId=lt-01b2c565029b5bf2a,Version=1"
  - ami: "ami-0d6d3e4f081c69f42"
    type: "t3a.small"
    name: "amd64:debian-stretch"
    username: "admin"
    python_interpreter: "/usr/bin/python3"
    launch_template: "LaunchTemplateId=lt-01b2c565029b5bf2a,Version=1"
  ```
  All spun instances will be named with the specified `name` variable in `instances` prefixed by the prefix `provision_host_prefix`
  defined in [group_vars/localhost/main.yml](group_vars/localhost/main.yml). 
  
  Once all instances are running, `inventory.ec2` file will be created to be used as inventory when running the harvest tests.
  To be able to connect to the spun instances, `ec2_private_key_file` will be used

* [terminate.yml](terminate.yml): Terminate instances. 

* [install-python.yml](install-python.yml): Install python (useful for instances that don't have python installed bvy default)
  

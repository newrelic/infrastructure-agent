#!/bin/bash

PLATFORMS=$1

echo "
---
dependency:
  name: galaxy
driver:
  name: docker
platforms:" > "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"


# this will collide with al-2022 🤔
if [[ "$PLATFORMS" == *"al-2"* ]]; then
echo "
  - name: al-2
    image: al-2
    dockerfile: al2.Dockerfile
    privileged: true
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"al-2022"* ]]; then
echo "
  - name: al-2022
    image: al-2022
    dockerfile: al2022.Dockerfile
    privileged: true
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"centos-7"* ]]; then
echo "
  - name: centos-7
    image: centos-7
    dockerfile: centos7.Dockerfile
    privileged: true
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"centos-8"* ]]; then
echo "
  - name: centos-8
    image: centos-8
    dockerfile: centos8.Dockerfile
    privileged: true
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"debian-bullseye"* ]]; then
echo "
  - name: debian-bullseye
    image: debian-bullseye
    command: \"/sbin/init\"
    privileged: true
    dockerfile: debian-bullseye.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"debian-buster"* ]]; then
echo "
  - name: debian-buster
    image: debian-buster
    command: \"/sbin/init\"
    privileged: true
    dockerfile: debian-buster.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"redhat-8"* ]]; then
echo "
  - name: redhat-8
    image: redhat-8
    privileged: true
    dockerfile: redhat8.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"redhat-9"* ]]; then
echo "
  - name: redhat-9
    image: redhat-9
    privileged: true
    dockerfile: redhat9.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"suse-15.2"* ]]; then
echo "
  - name: suse15.2
    image: suse15.2
    privileged: true
    dockerfile: suse15.2.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"suse-15.3"* ]]; then
echo "
  - name: suse15.3
    image: suse15.3
    privileged: true
    dockerfile: suse15.3.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"suse-15.4"* ]]; then
echo "
  - name: suse15.4
    image: suse15.4
    privileged: true
    dockerfile: suse15.4.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi


if [[ "$PLATFORMS" == *"ubuntu-1604"* ]]; then
echo "
  - name: ubuntu-1604
    image: ubuntu-1604
    command: \"/sbin/init\"
    privileged: true
    dockerfile: ubuntu1604.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"ubuntu-1804"* ]]; then
echo "
  - name: ubuntu-1804
    image: ubuntu-1804
    command: \"/sbin/init\"
    privileged: true
    dockerfile: ubuntu1804.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"ubuntu-2004"* ]]; then
echo "
  - name: ubuntu-2004
    image: ubuntu-2004
    command: \"/sbin/init\"
    privileged: true
    dockerfile: ubuntu2004.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

if [[ "$PLATFORMS" == *"ubuntu-2204"* ]]; then
echo "
  - name: ubuntu-2204
    image: ubuntu-2204
    command: \"/sbin/init\"
    privileged: true
    dockerfile: ubuntu2204.Dockerfile
    environment: { container: docker }
    groups:
      - testing_hosts_linux" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
fi

echo "
provisioner:
  name: ansible
  inventory:
    host_vars:
      al-2:
        ansible_python_interpreter: python
      al-2022:
        ansible_python_interpreter: python3
      centos-7:
        ansible_python_interpreter: python
      centos-8:
        ansible_python_interpreter: python3
      debian-bullseye:
        ansible_python_interpreter: python3
      debian-buster:
        ansible_python_interpreter: python3
      redhat-8:
        ansible_python_interpreter: python3
      redhat-9:
        ansible_python_interpreter: python3
      suse15.2:
        ansible_python_interpreter: python3
      suse15.3:
        ansible_python_interpreter: python3
      suse15.4:
        ansible_python_interpreter: python3
      ubuntu-1604:
        ansible_python_interpreter: python3
      ubuntu-1804:
        ansible_python_interpreter: python3
      ubuntu-2004:
        ansible_python_interpreter: python3
      ubuntu-2204:
        ansible_python_interpreter: python3
  env:
    ANSIBLE_ROLES_PATH: \"../../roles\"
verifier:
  name: ansible
" >> "${GITHUB_ACTION_PATH}/molecule/default/molecule.yml"
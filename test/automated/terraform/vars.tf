variable "region" {
  default = "us-east-2"
}

variable "accountId" {
  default = "018789649883"
}

variable "vpc_id" {
  default = "vpc-0a3c00f5dc8645fe0"
}

variable "security_group_id" {
  default = "sg-044ef7bc34691164a"
}

variable "vpc_subnet_ec2" {
  default = "subnet-09b64de757828cdd4"
}

variable "cluster_name" {
  default = "caos_prerelease"
}

#######################################
# Task definition
variable "task_command" {
  default = [
    "test/automated-run"
  ]
}

variable "efs_volume_mount_point" {
  default = "/srv/runner/inventory"
}

variable "efs_volume_name" {
  default = "pipeline-shared"
}

variable "secret_name_ssh" {
  default = "caos/canaries/ssh_key-UBSKNA"
}

variable "secret_name_license" {
  default = "caos/canaries/license-f9eYwe"
}

variable "secret_name_license_canaries" {
  default = "caos/canaries/license_canaries-1DCE1L"
}

variable "secret_name_account" {
  default = "caos/canaries/account-kKFMGP"
}

variable "secret_name_api" {
  default = "caos/canaries/api-9q0NPb"
}

variable "secret_name_windows_password" {
  default = "caos/canaries/windows-gTLIiF"
}

variable "secret_name_macstadium_user" {
  default = "caos/canaries/macstadium_user-QXCSKB"
}

variable "secret_name_macstadium_pass" {
  default = "caos/canaries/macstadium_pass-DvAHye"
}

variable "secret_name_macstadium_sudo_pass" {
  default = "caos/canaries/macstadium_sudo_pass-4h4DKS"
}

variable "secret_name_nr_api_key" {
  default = "caos/canaries/nr_api_key-xadBYJ"
}

variable "task_container_image" {
  default = "ghcr.io/newrelic/infrastructure-agent-ci-runner:latest"
}

variable "task_logs_group" {
  default = "/ecs/test-prerelease"
}

variable "task_container_name" {
  default = "test-prerelease"
}

variable "task_name_prefix" {
  default = "test-prerelease"
}

variable "task_entrypoint" {
  default = [
  ]
}

variable "task_logs_prefix" {
  default = "ecs"
}

variable "task_container_cpu" {
  default = "4096"
}

variable "task_container_memory" {
  default = "30720"
}

variable "task_container_memory_reservation" {
  default = "2048"
}

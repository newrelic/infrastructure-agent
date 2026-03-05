variable "region" {
  default = "us-east-2"
}

variable "accountId" {
  default = "971422713139"
}

variable "vpc_id" {
  default = "vpc-0bc4f5a177616dbdf"
}

variable "vpc_subnet" {
  default = "subnet-0c2046d7a0595aa2c"
}

variable "cluster_name" {
  default = "caos_infra_agent"
}

####

#######################################
# Task definition
#######################################
variable "task_command" {
  default = [
    "test/automated-run"
  ]
}

variable "efs_volume_mount_point" {
  default = "/srv/runner/inventory"
}

variable "efs_volume_name" {
  default = "shared-infra-agent"
}

variable "additional_efs_security_group_rules" {
    default = [
    {
      type                     = "ingress"
      from_port                = 0
      to_port                  = 65535
      protocol                 = "tcp"
      cidr_blocks              = ["10.10.0.0/24"]
      description              = "Allow ingress traffic to EFS from trusted subnet"
    }
  ]
}

variable "canaries_security_group" {
  default = "sg-075f379cc5612e984"
}

variable "secret_name_ssh" {
  default = "caos/canaries/ssh_key-0ZnfDz"
}

variable "secret_name_license" {
  default = "caos/canaries/license-PJP9uk"
}

variable "secret_name_license_canaries" {
  default = "caos/canaries/license_canaries-VVcT0q"
}

variable "secret_name_license_canaries_A2Q_1" {
  default = "caos/canaries/license_canaries_A2Q_1-HYhssG"
}

variable "secret_name_license_canaries_A2Q_2" {
  default = "caos/canaries/license_canaries_A2Q_2-Y09LU5"
}

variable "secret_name_license_keys_canaries_A2Q" {
  default = "caos/canaries/license_keys_canaries_A2Q-ozAvrS"
}

variable "secret_name_account" {
  default = "caos/canaries/account-EOzJkq"
}

variable "secret_name_api" {
  default = "caos/canaries/api-rnTPmn"
}

variable "secret_name_windows_password" {
  default = "caos/canaries/windows-rJ9Tep"
}

variable "secret_name_macstadium_user" {
  default = "caos/canaries/macstadium_user-2s39p7"
}

variable "secret_name_macstadium_pass" {
  default = "caos/canaries/macstadium_pass-H1EYQd"
}

variable "secret_name_macstadium_sudo_pass" {
  default = "caos/canaries/macstadium_sudo_pass-dhVHqK"
}

variable "secret_name_nr_api_key" {
  default = "caos/canaries/nr_api_key-gY3vau"
}

variable "task_container_image" {
  default = "ghcr.io/newrelic/fargate-runner-action:latest"
}

variable "task_logs_group" {
  default = "/ecs/test-prerelease-infra-agent"
}

variable "task_container_name" {
  default = "test-prerelease"
}

variable "task_name_prefix" {
  default = "infra-agent"
}

variable "task_logs_prefix" {
  default = "ecs-infra-agent"
}

variable "s3_bucket" {
  default = "arn:aws:s3:::automation-pipeline-terraform-states"
}

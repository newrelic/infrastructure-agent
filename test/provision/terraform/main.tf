variable "nr_license_key" {
  default = ""
}
variable "otlp_endpoint" {
  default = ""
}
variable "pvt_key" {
  default = ""
}
variable "windows_password" {
  default = ""
}

variable "platform"{
  default = ""
}

variable "ssh_pub_key" {
  default = ""
}
variable "ansible_playbook" {
  default = ""
}

variable "region" {
  default = "us-east-2"
}

provider "aws" {
  region = var.region
}

variable "inventory_output" {
  default = "./inventory.ec2"
}

variable "windows_ec2" {
  default = ""
}

variable "linux_ec2_amd" {
  default = ""
}

variable "linux_ec2_arm" {
  default = ""
}

variable "ec2_prefix" {
  default = ""
}

variable "is_A2Q" {
  default = false
}

locals {
    filtered_ec2 = var.platform == "windows" ? var.windows_ec2 : flatten([var.linux_ec2_amd, var.linux_ec2_arm])
}

module "env-provisioner" {
  source             = "git::https://github.com/newrelic-experimental/env-provisioner//terraform/otel-ec2?ref=test-rhel-10"
  ec2_prefix         = var.ec2_prefix
  ec2_filters        = local.filtered_ec2
  ec2_delimiter      = "-"
  is_A2Q             = var.is_A2Q
  nr_license_key     = var.nr_license_key
  otlp_endpoint      = var.otlp_endpoint
  pvt_key            = var.pvt_key
  windows_password   = var.windows_password
  ssh_pub_key        = var.ssh_pub_key
  inventory_template = "${path.module}/inventory.tmpl"
  inventory_output   = var.inventory_output
  ansible_playbook   = var.ansible_playbook
}


output "check_vars" {
    value = "${var.inventory_output}"
}

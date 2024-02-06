variable "ec2_infra_agents" {
  default = {}
}
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

module "env-provisioner" {
  source             = "git::https://github.com/newrelic-experimental/env-provisioner//terraform/otel-ec2"
  ec2_otels         =    {for key, val in var.ec2_infra_agents:
                            key => val if val.platform == var.platform || var.platform == "all"}

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

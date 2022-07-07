#########################################
# State Backend
#########################################
terraform {
  backend "s3" {
    bucket = "automation-pipeline-terraform-state"
    key    = "state"
    region = "us-east-2"
  }
}

#########################################
# ECS Cluster
#########################################
module "ecs" {
  source = "registry.terraform.io/terraform-aws-modules/ecs/aws"
  version = "3.5.0"

  name                               = var.cluster_name
  capacity_providers                 = ["FARGATE"]
  default_capacity_provider_strategy = [
    {
      capacity_provider = "FARGATE"
    }
  ]

  tags = {
    owning_team = "CAOS"
  }
}

#########################################
# Log group
#########################################
module "cloudwatch_log-group" {
  source  = "registry.terraform.io/terraform-aws-modules/cloudwatch/aws//modules/log-group"
  version = "3.2.0"

  name              = var.task_logs_group
  retention_in_days = 1
}

#########################################
# IAM Policy Fargate
#########################################
module "iam_policy_fargate" {
  source  = "registry.terraform.io/terraform-aws-modules/iam/aws//modules/iam-policy"
  version = "5.1.0"

  name        = "test_prerelease"
  path        = "/"
  description = "Policy for Fargate task to provision ec2 instances and logwatch"

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": "ec2:*",
            "Effect": "Allow",
            "Resource": "*"
        }
    ]
}
EOF

  tags = {
    owning_team = "CAOS"
  }
}

module "iam_assumable_role_custom" {
  source  = "registry.terraform.io/terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  version = "5.1.0"

  create_role = true

  role_name         = "test-prerelease-fargate"
  role_requires_mfa = false

  custom_role_trust_policy = data.aws_iam_policy_document.custom_trust_policy.json

  custom_role_policy_arns = [
    module.iam_policy_fargate.arn
  ]
}

data "aws_iam_policy_document" "custom_trust_policy" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

#########################################
# ECS Task definition
#########################################
module "ecs-fargate-task-definition" {
  source  = "registry.terraform.io/cn-terraform/ecs-fargate-task-definition/aws"
  version = "1.0.29"

  command                      = var.task_command
  container_image              = var.task_container_image
  container_name               = var.task_container_name
  entrypoint                   = var.task_entrypoint
  name_prefix                  = var.task_name_prefix
  port_mappings                = []
  container_cpu                = var.task_container_cpu
  container_memory             = var.task_container_memory
  container_memory_reservation = var.task_container_memory_reservation
  volumes                      = [
    {
      name                        = var.efs_volume_name,
      host_path                   = "",
      docker_volume_configuration = []
      efs_volume_configuration    = [
        {
          file_system_id          = module.efs.id,
          root_directory          = "/"
          transit_encryption      = "DISABLED"
          authorization_config    = []
          transit_encryption_port = null
        }
      ]
    }
  ]

  mount_points = [
    {
      containerPath = var.efs_volume_mount_point
      sourceVolume  = var.efs_volume_name
      readOnly      = false
    }
  ]

  task_role_arn                           = module.iam_assumable_role_custom.iam_role_arn
  ecs_task_execution_role_custom_policies = [
    jsonencode(
      {
        "Version" : "2012-10-17",
        "Statement" : [

          {
            "Effect" : "Allow",
            "Action" : [
              "secretsmanager:GetSecretValue"
            ],
            "Resource" : [
              "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_ssh}",
              "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_license}",
              "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_account}",
              "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_api}",
              "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_windows_password}"
            ]
          }
        ]
      }
    )
  ]

  secrets = [
    {
      "name" : "SSH_KEY",
      "valueFrom" : "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_ssh}"
    },
    {
      "name" : "NR_LICENSE_KEY",
      "valueFrom" : "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_license}"
    },
    {
      "name" : "NEW_RELIC_ACCOUNT_ID",
      "valueFrom" : "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_account}"
    },
    {
      "name" : "NEW_RELIC_API_KEY",
      "valueFrom" : "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_api}"
    },
    {
      "name" : "ANSIBLE_PASSWORD_WINDOWS",
      "valueFrom" : "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name_windows_password}"
    }
  ]
  log_configuration = {
    "logDriver" : "awslogs",
    "secretOptions" : null,
    "options" : {
      "awslogs-group" : var.task_logs_group,
      "awslogs-region" : var.region,
      "awslogs-stream-prefix" : var.task_logs_prefix
    }
  }
}

#########################################
# IAM Policy Task execution
#########################################
module "iam_policy_task_execution" {
  source  = "registry.terraform.io/terraform-aws-modules/iam/aws//modules/iam-policy"
  version = "5.1.0"

  name        = "ecs_task_execution_policy"
  path        = "/"
  description = "Policy for Fargate task execution"

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "ecs:RunTask",
                "logs:GetLogEvents",
                "ecs:DescribeTasks",
                "ecs:DescribeTaskDefinition"
            ],
            "Resource": "*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": "iam:PassRole",
            "Resource": "*",
            "Condition": {
                "StringLike": {
                    "iam:PassedToService": "ecs-tasks.amazonaws.com"
                }
            }
        }
    ]
}
EOF

  tags = {
    owning_team = "CAOS"
  }
}

#########################################
# Create IAM assumable role OIDC
#########################################
module "iam_iam-assumable-role-with-oidc" {
  source                         = "registry.terraform.io/terraform-aws-modules/iam/aws//modules/iam-assumable-role-with-oidc"
  version                        = "5.1.0"
  # insert the 3 required variables here
  provider_url                   = "https://token.actions.githubusercontent.com"
  oidc_fully_qualified_audiences = [
    "sts.amazonaws.com"
  ]
  oidc_subjects_with_wildcards = [
    "repo:newrelic/infrastructure-agent:*"
  ]
  create_role           = true
  role_name             = "caos-pipeline-oidc"
  force_detach_policies = true
  max_session_duration  = 43200
  aws_account_id        = "018789649883"
  role_policy_arns      = [
    module.iam_policy_task_execution.arn
  ]
  tags = {
    "owning_team" : "CAOS"
  }
}

#########################################
# Create EFS Volumne
#########################################

module "efs" {
  source  = "registry.terraform.io/cloudposse/efs/aws"
  version = "0.32.7"
  # insert the 18 required variables here

  name            = var.efs_volume_name
  vpc_id          = var.vpc_id
  subnets         = [var.vpc_subnet_ec2]
  region          = var.region
  security_groups = [var.security_group_id]

}


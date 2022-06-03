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
# AMI
#########################################
module "iam_policy" {
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
            "Effect": "Allow",
            "Action": [
                "ecr:GetAuthorizationToken",
                "ecr:BatchCheckLayerAvailability",
                "ecr:GetDownloadUrlForLayer",
                "ecr:BatchGetImage",
                "logs:CreateLogStream",
                "logs:PutLogEvents"
            ],
            "Resource": "*"
        },
        {
            "Action": "ec2:*",
            "Effect": "Allow",
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": [
              "secretsmanager:GetSecretValue"
            ],
            "Resource": [
              "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name}"
            ]
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
    module.iam_policy.arn
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
  task_role_arn                = module.iam_assumable_role_custom.iam_role_arn
  secrets                      = [
    {
      "name" : "SSH_KEY",
      "valueFrom" : "arn:aws:secretsmanager:${var.region}:${var.accountId}:secret:${var.secret_name}"
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

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.40"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.27"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
  }

  backend "s3" {
    bucket         = "nexusops-terraform-state"
    key            = "infrastructure/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "nexusops-terraform-lock"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "nexusops"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name   = "nexusops-${var.environment}"
  azs    = slice(data.aws_availability_zones.available.names, 0, 3)

  tags = {
    Project     = "nexusops"
    Environment = var.environment
  }
}

# -------------------------------------------------------------------
# VPC
# -------------------------------------------------------------------
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.5"

  name = "${local.name}-vpc"
  cidr = var.vpc_cidr

  azs              = local.azs
  private_subnets  = [for i, az in local.azs : cidrsubnet(var.vpc_cidr, 4, i)]
  public_subnets   = [for i, az in local.azs : cidrsubnet(var.vpc_cidr, 4, i + 4)]
  database_subnets = [for i, az in local.azs : cidrsubnet(var.vpc_cidr, 4, i + 8)]

  enable_nat_gateway   = true
  single_nat_gateway   = var.environment != "production"
  enable_dns_hostnames = true
  enable_dns_support   = true

  create_database_subnet_group       = true
  create_database_subnet_route_table = true

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = 1
    "kubernetes.io/cluster/${local.name}" = "owned"
  }

  public_subnet_tags = {
    "kubernetes.io/role/elb" = 1
    "kubernetes.io/cluster/${local.name}" = "owned"
  }

  tags = local.tags
}

# -------------------------------------------------------------------
# EKS Cluster
# -------------------------------------------------------------------
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.8"

  cluster_name    = local.name
  cluster_version = var.eks_cluster_version

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_endpoint_public_access  = true
  cluster_endpoint_private_access = true

  enable_cluster_creator_admin_permissions = true

  cluster_addons = {
    coredns = {
      most_recent = true
    }
    kube-proxy = {
      most_recent = true
    }
    vpc-cni = {
      most_recent              = true
      service_account_role_arn = module.vpc_cni_irsa.iam_role_arn
    }
    aws-ebs-csi-driver = {
      most_recent              = true
      service_account_role_arn = module.ebs_csi_irsa.iam_role_arn
    }
  }

  eks_managed_node_groups = {
    default = {
      name           = "${local.name}-default"
      instance_types = var.node_instance_types
      capacity_type  = var.environment == "production" ? "ON_DEMAND" : "SPOT"

      min_size     = var.node_min_size
      max_size     = var.node_max_size
      desired_size = var.node_desired_size

      disk_size = 50

      labels = {
        role = "general"
      }

      tags = local.tags
    }

    runners = {
      name           = "${local.name}-runners"
      instance_types = var.runner_instance_types
      capacity_type  = "ON_DEMAND"

      min_size     = var.runner_min_size
      max_size     = var.runner_max_size
      desired_size = var.runner_desired_size

      disk_size = 100

      labels = {
        role = "runner"
      }

      taints = [{
        key    = "workload"
        value  = "runner"
        effect = "NO_SCHEDULE"
      }]

      tags = local.tags
    }
  }

  tags = local.tags
}

# -------------------------------------------------------------------
# IRSA for VPC CNI
# -------------------------------------------------------------------
module "vpc_cni_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.37"

  role_name             = "${local.name}-vpc-cni"
  attach_vpc_cni_policy = true
  vpc_cni_enable_ipv4   = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:aws-node"]
    }
  }

  tags = local.tags
}

# -------------------------------------------------------------------
# IRSA for EBS CSI Driver
# -------------------------------------------------------------------
module "ebs_csi_irsa" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.37"

  role_name             = "${local.name}-ebs-csi"
  attach_ebs_csi_policy = true

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }

  tags = local.tags
}

# -------------------------------------------------------------------
# RDS PostgreSQL
# -------------------------------------------------------------------
module "rds" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.5"

  identifier = "${local.name}-postgres"

  engine               = "postgres"
  engine_version       = var.postgres_engine_version
  family               = "postgres16"
  major_engine_version = "16"
  instance_class       = var.rds_instance_class

  allocated_storage     = var.rds_allocated_storage
  max_allocated_storage = var.rds_max_allocated_storage
  storage_encrypted     = true
  storage_type          = "gp3"

  db_name  = "nexusops"
  username = "nexusops"
  port     = 5432

  multi_az               = var.environment == "production"
  db_subnet_group_name   = module.vpc.database_subnet_group_name
  vpc_security_group_ids = [module.rds_sg.security_group_id]

  backup_retention_period = var.environment == "production" ? 30 : 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  deletion_protection = var.environment == "production"
  skip_final_snapshot = var.environment != "production"

  performance_insights_enabled    = true
  create_monitoring_role          = true
  monitoring_role_name            = "${local.name}-rds-monitoring"
  monitoring_interval             = 60

  parameters = [
    {
      name  = "shared_preload_libraries"
      value = "pg_stat_statements"
    },
    {
      name  = "log_min_duration_statement"
      value = "1000"
    }
  ]

  tags = local.tags
}

module "rds_sg" {
  source  = "terraform-aws-modules/security-group/aws"
  version = "~> 5.1"

  name        = "${local.name}-rds"
  description = "Security group for NexusOps RDS PostgreSQL"
  vpc_id      = module.vpc.vpc_id

  ingress_with_cidr_blocks = [
    {
      from_port   = 5432
      to_port     = 5432
      protocol    = "tcp"
      cidr_blocks = module.vpc.vpc_cidr_block
      description = "PostgreSQL from VPC"
    }
  ]

  tags = local.tags
}

# -------------------------------------------------------------------
# ElastiCache Redis
# -------------------------------------------------------------------
resource "aws_elasticache_subnet_group" "redis" {
  name       = "${local.name}-redis"
  subnet_ids = module.vpc.private_subnets
}

resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "${local.name}-redis"
  description          = "NexusOps Redis cluster"

  engine               = "redis"
  engine_version       = var.redis_engine_version
  node_type            = var.elasticache_node_type
  num_cache_clusters   = var.environment == "production" ? 2 : 1
  port                 = 6379
  parameter_group_name = "default.redis7"

  subnet_group_name  = aws_elasticache_subnet_group.redis.name
  security_group_ids = [module.redis_sg.security_group_id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = var.redis_auth_token

  automatic_failover_enabled = var.environment == "production"
  multi_az_enabled           = var.environment == "production"

  snapshot_retention_limit = var.environment == "production" ? 7 : 1
  snapshot_window          = "05:00-06:00"
  maintenance_window       = "Mon:06:00-Mon:07:00"

  tags = local.tags
}

module "redis_sg" {
  source  = "terraform-aws-modules/security-group/aws"
  version = "~> 5.1"

  name        = "${local.name}-redis"
  description = "Security group for NexusOps ElastiCache Redis"
  vpc_id      = module.vpc.vpc_id

  ingress_with_cidr_blocks = [
    {
      from_port   = 6379
      to_port     = 6379
      protocol    = "tcp"
      cidr_blocks = module.vpc.vpc_cidr_block
      description = "Redis from VPC"
    }
  ]

  tags = local.tags
}

# -------------------------------------------------------------------
# ALB
# -------------------------------------------------------------------
module "alb" {
  source  = "terraform-aws-modules/alb/aws"
  version = "~> 9.7"

  name               = "${local.name}-alb"
  load_balancer_type = "application"
  vpc_id             = module.vpc.vpc_id
  subnets            = module.vpc.public_subnets
  security_group_rules = {
    ingress_http = {
      type        = "ingress"
      from_port   = 80
      to_port     = 80
      protocol    = "tcp"
      cidr_blocks = ["0.0.0.0/0"]
      description = "HTTP"
    }
    ingress_https = {
      type        = "ingress"
      from_port   = 443
      to_port     = 443
      protocol    = "tcp"
      cidr_blocks = ["0.0.0.0/0"]
      description = "HTTPS"
    }
    egress_all = {
      type        = "egress"
      from_port   = 0
      to_port     = 0
      protocol    = "-1"
      cidr_blocks = ["0.0.0.0/0"]
      description = "Allow all outbound"
    }
  }

  listeners = {
    http = {
      port     = 80
      protocol = "HTTP"
      redirect = {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }
    https = {
      port            = 443
      protocol        = "HTTPS"
      certificate_arn = var.acm_certificate_arn
      ssl_policy      = "ELBSecurityPolicy-TLS13-1-2-2021-06"

      forward = {
        target_group_key = "api"
      }
    }
  }

  target_groups = {
    api = {
      name             = "${local.name}-api"
      protocol         = "HTTP"
      port             = 8080
      target_type      = "ip"
      create_attachment = false

      health_check = {
        enabled             = true
        healthy_threshold   = 2
        unhealthy_threshold = 3
        interval            = 30
        path                = "/health"
        port                = "traffic-port"
        protocol            = "HTTP"
        timeout             = 5
      }
    }
    frontend = {
      name             = "${local.name}-frontend"
      protocol         = "HTTP"
      port             = 3000
      target_type      = "ip"
      create_attachment = false

      health_check = {
        enabled             = true
        healthy_threshold   = 2
        unhealthy_threshold = 3
        interval            = 30
        path                = "/"
        port                = "traffic-port"
        protocol            = "HTTP"
        timeout             = 5
      }
    }
  }

  tags = local.tags
}

# -------------------------------------------------------------------
# ECR Repositories
# -------------------------------------------------------------------
resource "aws_ecr_repository" "services" {
  for_each = toset(["api", "runner", "proxy", "frontend"])

  name                 = "nexusops/${each.key}"
  image_tag_mutability = "IMMUTABLE"
  force_delete         = var.environment != "production"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = local.tags
}

resource "aws_ecr_lifecycle_policy" "services" {
  for_each = aws_ecr_repository.services

  repository = each.value.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 30 tagged images"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["v"]
          countType     = "imageCountMoreThan"
          countNumber   = 30
        }
        action = {
          type = "expire"
        }
      },
      {
        rulePriority = 2
        description  = "Expire untagged images older than 7 days"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 7
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment (development, staging, production)"
  type        = string
  default     = "staging"

  validation {
    condition     = contains(["development", "staging", "production"], var.environment)
    error_message = "Environment must be one of: development, staging, production."
  }
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

# -------------------------------------------------------------------
# EKS
# -------------------------------------------------------------------

variable "eks_cluster_version" {
  description = "Kubernetes version for the EKS cluster"
  type        = string
  default     = "1.29"
}

variable "node_instance_types" {
  description = "EC2 instance types for the default EKS node group"
  type        = list(string)
  default     = ["t3.large", "t3a.large"]
}

variable "node_min_size" {
  description = "Minimum number of nodes in the default node group"
  type        = number
  default     = 2
}

variable "node_max_size" {
  description = "Maximum number of nodes in the default node group"
  type        = number
  default     = 10
}

variable "node_desired_size" {
  description = "Desired number of nodes in the default node group"
  type        = number
  default     = 3
}

variable "runner_instance_types" {
  description = "EC2 instance types for the runner EKS node group"
  type        = list(string)
  default     = ["c5.xlarge", "c5a.xlarge"]
}

variable "runner_min_size" {
  description = "Minimum number of nodes in the runner node group"
  type        = number
  default     = 1
}

variable "runner_max_size" {
  description = "Maximum number of nodes in the runner node group"
  type        = number
  default     = 20
}

variable "runner_desired_size" {
  description = "Desired number of nodes in the runner node group"
  type        = number
  default     = 2
}

# -------------------------------------------------------------------
# RDS
# -------------------------------------------------------------------

variable "postgres_engine_version" {
  description = "PostgreSQL engine version"
  type        = string
  default     = "16.2"
}

variable "rds_instance_class" {
  description = "RDS instance class for PostgreSQL"
  type        = string
  default     = "db.t4g.medium"
}

variable "rds_allocated_storage" {
  description = "Allocated storage for RDS in GB"
  type        = number
  default     = 20
}

variable "rds_max_allocated_storage" {
  description = "Maximum allocated storage for RDS autoscaling in GB"
  type        = number
  default     = 100
}

# -------------------------------------------------------------------
# ElastiCache
# -------------------------------------------------------------------

variable "redis_engine_version" {
  description = "Redis engine version"
  type        = string
  default     = "7.1"
}

variable "elasticache_node_type" {
  description = "ElastiCache node type for Redis"
  type        = string
  default     = "cache.t4g.medium"
}

variable "redis_auth_token" {
  description = "Auth token for Redis (must be at least 16 characters)"
  type        = string
  sensitive   = true
}

# -------------------------------------------------------------------
# ALB / TLS
# -------------------------------------------------------------------

variable "acm_certificate_arn" {
  description = "ARN of the ACM certificate for HTTPS on the ALB"
  type        = string
}

variable "domain_name" {
  description = "Primary domain name for the application"
  type        = string
  default     = "nexusops.example.com"
}

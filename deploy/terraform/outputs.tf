output "vpc_id" {
  description = "ID of the VPC"
  value       = module.vpc.vpc_id
}

output "vpc_cidr_block" {
  description = "CIDR block of the VPC"
  value       = module.vpc.vpc_cidr_block
}

output "private_subnet_ids" {
  description = "IDs of the private subnets"
  value       = module.vpc.private_subnets
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = module.vpc.public_subnets
}

output "database_subnet_ids" {
  description = "IDs of the database subnets"
  value       = module.vpc.database_subnets
}

# -------------------------------------------------------------------
# EKS
# -------------------------------------------------------------------

output "eks_cluster_name" {
  description = "Name of the EKS cluster"
  value       = module.eks.cluster_name
}

output "eks_cluster_endpoint" {
  description = "Endpoint for the EKS cluster API server"
  value       = module.eks.cluster_endpoint
}

output "eks_cluster_certificate_authority" {
  description = "Base64 encoded certificate data for the EKS cluster"
  value       = module.eks.cluster_certificate_authority_data
  sensitive   = true
}

output "eks_cluster_oidc_issuer_url" {
  description = "OIDC issuer URL of the EKS cluster"
  value       = module.eks.cluster_oidc_issuer_url
}

output "eks_kubeconfig_command" {
  description = "Command to configure kubectl for the EKS cluster"
  value       = "aws eks update-kubeconfig --region ${var.aws_region} --name ${module.eks.cluster_name}"
}

# -------------------------------------------------------------------
# RDS
# -------------------------------------------------------------------

output "rds_endpoint" {
  description = "Endpoint of the RDS PostgreSQL instance"
  value       = module.rds.db_instance_endpoint
}

output "rds_instance_id" {
  description = "ID of the RDS instance"
  value       = module.rds.db_instance_identifier
}

output "rds_database_name" {
  description = "Name of the default database"
  value       = module.rds.db_instance_name
}

output "rds_master_username" {
  description = "Master username for the RDS instance"
  value       = module.rds.db_instance_username
  sensitive   = true
}

output "rds_master_password_secret_arn" {
  description = "ARN of the Secrets Manager secret containing the RDS master password"
  value       = module.rds.db_instance_master_user_secret_arn
}

# -------------------------------------------------------------------
# ElastiCache
# -------------------------------------------------------------------

output "redis_endpoint" {
  description = "Primary endpoint of the Redis replication group"
  value       = aws_elasticache_replication_group.redis.primary_endpoint_address
}

output "redis_reader_endpoint" {
  description = "Reader endpoint of the Redis replication group"
  value       = aws_elasticache_replication_group.redis.reader_endpoint_address
}

output "redis_port" {
  description = "Port of the Redis cluster"
  value       = aws_elasticache_replication_group.redis.port
}

# -------------------------------------------------------------------
# ALB
# -------------------------------------------------------------------

output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer"
  value       = module.alb.dns_name
}

output "alb_zone_id" {
  description = "Route53 zone ID of the ALB"
  value       = module.alb.zone_id
}

output "alb_arn" {
  description = "ARN of the Application Load Balancer"
  value       = module.alb.arn
}

# -------------------------------------------------------------------
# ECR
# -------------------------------------------------------------------

output "ecr_repository_urls" {
  description = "Map of ECR repository URLs"
  value       = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}

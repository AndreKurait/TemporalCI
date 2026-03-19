variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "temporalci"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "ecr_repo_name" {
  description = "ECR repository name"
  type        = string
  default     = "temporalci"
}

# --- RDS (provisioned via ACK, but Terraform manages the subnet group) ---

variable "rds_instance_class" {
  description = "RDS instance class for Temporal DB"
  type        = string
  default     = "db.t4g.medium"
}

variable "rds_subnet_group_name" {
  description = "DB subnet group name"
  type        = string
  default     = ""
}

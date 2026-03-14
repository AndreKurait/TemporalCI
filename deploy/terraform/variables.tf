variable "cluster_name" {
  description = "Name of the EKS cluster"
  type        = string
  default     = "temporalci"
}

variable "region" {
  description = "AWS region"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID for the EKS cluster"
  type        = string
}

variable "private_subnet_ids" {
  description = "List of private subnet IDs for the EKS cluster"
  type        = list(string)
}

variable "ecr_repo_name" {
  description = "ECR repository name"
  type        = string
  default     = "temporalci"
}

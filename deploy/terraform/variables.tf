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

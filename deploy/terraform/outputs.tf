output "cluster_name" {
  value = aws_eks_cluster.this.name
}

output "cluster_endpoint" {
  value = aws_eks_cluster.this.endpoint
}

output "cluster_certificate_authority" {
  value = aws_eks_cluster.this.certificate_authority[0].data
}

output "ecr_repository_url" {
  value = aws_ecr_repository.temporalci.repository_url
}

output "vpc_id" {
  value = aws_vpc.this.id
}

output "ci_logs_bucket" {
  value = aws_s3_bucket.ci_logs.bucket
}

output "rds_subnet_group" {
  value = aws_db_subnet_group.temporal.name
}

output "ack_role_arn" {
  value = aws_iam_role.ack.arn
}

output "github_actions_role_arn" {
  value = aws_iam_role.github_actions.arn
}

output "ecr_registry" {
  description = "ECR registry URL (without repo name) for helm --set image.registry="
  value       = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.region}.amazonaws.com"
}

output "node_instance_profile" {
  value = aws_iam_instance_profile.node.name
}
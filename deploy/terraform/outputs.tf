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

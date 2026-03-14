resource "aws_eks_cluster" "this" {
  name     = var.cluster_name
  role_arn = aws_iam_role.eks_cluster.arn

  vpc_config {
    subnet_ids              = var.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = true
  }

  compute_config {
    enabled       = true
    node_pools    = ["system", "ci-jobs"]
    node_role_arn = aws_iam_role.eks_node.arn
  }

  kubernetes_network_config {
    elastic_load_balancing {
      enabled = true
    }
  }

  storage_config {
    block_storage {
      enabled = true
    }
  }

  access_config {
    authentication_mode = "API_AND_CONFIG_MAP"
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy,
    aws_iam_role_policy_attachment.eks_compute_policy,
    aws_iam_role_policy_attachment.eks_block_storage_policy,
    aws_iam_role_policy_attachment.eks_load_balancing_policy,
    aws_iam_role_policy_attachment.eks_networking_policy,
  ]
}

resource "aws_eks_node_pool" "system" {
  cluster_name = aws_eks_cluster.this.name
  node_pool_name = "system"

  node_role_arn = aws_iam_role.eks_node.arn

  depends_on = [aws_eks_cluster.this]
}

resource "aws_eks_node_pool" "ci_jobs" {
  cluster_name   = aws_eks_cluster.this.name
  node_pool_name = "ci-jobs"

  node_role_arn = aws_iam_role.eks_node.arn

  taint {
    key    = "workload"
    value  = "ci-job"
    effect = "NO_SCHEDULE"
  }

  depends_on = [aws_eks_cluster.this]
}

resource "aws_eks_addon" "secrets_store_csi" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "secrets-store-csi-driver-provider-aws"

  depends_on = [aws_eks_cluster.this]
}

resource "aws_eks_addon" "cloudwatch_observability" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "amazon-cloudwatch-observability"

  depends_on = [aws_eks_cluster.this]
}

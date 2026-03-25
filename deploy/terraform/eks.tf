# --- EKS Auto Mode Cluster ---

resource "aws_eks_cluster" "this" {
  name     = var.cluster_name
  role_arn = aws_iam_role.eks_cluster.arn

  bootstrap_self_managed_addons = false

  vpc_config {
    subnet_ids              = concat(aws_subnet.private[*].id, aws_subnet.public[*].id)
    endpoint_private_access = true
    endpoint_public_access  = true
  }

  # Auto Mode: EKS manages compute, scaling, bin-packing
  compute_config {
    enabled       = true
    node_pools    = ["general-purpose"]
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
    aws_nat_gateway.this,
  ]
}

# --- EKS Add-ons (managed by AWS, no node group dependency) ---

resource "aws_eks_addon" "secrets_store_csi" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "aws-secrets-store-csi-driver-provider"
}

resource "aws_eks_addon" "cloudwatch_observability" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "amazon-cloudwatch-observability"
  timeouts { create = "30m" }
}

resource "aws_eks_addon" "pod_identity" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "eks-pod-identity-agent"
}

# vpc-cni, kube-proxy, coredns are needed for managed node groups.
# (Auto Mode nodes have these built-in, but managed node groups do not.)

resource "aws_eks_addon" "vpc_cni" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "vpc-cni"
}

resource "aws_eks_addon" "kube_proxy" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "kube-proxy"
}

resource "aws_eks_addon" "coredns" {
  cluster_name = aws_eks_cluster.this.name
  addon_name   = "coredns"
}

resource "aws_eks_addon" "ebs_csi" {
  cluster_name             = aws_eks_cluster.this.name
  addon_name               = "aws-ebs-csi-driver"
  service_account_role_arn = aws_iam_role.ebs_csi.arn
}

# --- Access Entry ---

resource "aws_eks_access_entry" "node" {
  cluster_name  = aws_eks_cluster.this.name
  principal_arn = aws_iam_role.eks_node.arn
  type          = "EC2_LINUX"
}

resource "aws_eks_pod_identity_association" "ebs_csi" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = "kube-system"
  service_account = "ebs-csi-controller-sa"
  role_arn        = aws_iam_role.ebs_csi.arn
}

# --- ACK Capability (provisions AWS resources as K8s CRDs) ---

resource "aws_eks_addon" "ack_rds" {
  cluster_name             = aws_eks_cluster.this.name
  addon_name               = "ack-rds-controller"
  service_account_role_arn = aws_iam_role.ack.arn
}

resource "aws_eks_addon" "ack_s3" {
  cluster_name             = aws_eks_cluster.this.name
  addon_name               = "ack-s3-controller"
  service_account_role_arn = aws_iam_role.ack.arn
}

# --- Instance Profile for Node Role ---
# EKS Auto Mode's SLR lacks iam:AddRoleToInstanceProfile, so we pre-create it.

resource "aws_iam_instance_profile" "node" {
  name = "${var.cluster_name}-node"
  role = aws_iam_role.eks_node.name
}

# --- Managed Node Group (x86, for running TemporalCI workloads) ---

resource "aws_eks_node_group" "workers" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "workers"
  node_role_arn   = aws_iam_role.eks_node.arn
  subnet_ids      = aws_subnet.private[*].id

  instance_types = ["m6a.large", "c6a.large", "m5.large", "c5.large"]
  ami_type       = "AL2023_x86_64_STANDARD"
  disk_size      = 80

  scaling_config {
    desired_size = 2
    min_size     = 1
    max_size     = 4
  }

  update_config {
    max_unavailable = 1
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node_policy,
    aws_iam_role_policy_attachment.eks_worker_node_minimal_policy,
    aws_iam_role_policy_attachment.eks_cni_policy,
    aws_iam_role_policy_attachment.ecr_read_only,
    aws_iam_role_policy_attachment.eks_node_ssm,
  ]

  tags = {
    Project = "TemporalCI"
  }
}
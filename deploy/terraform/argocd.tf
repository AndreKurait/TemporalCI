# --- ArgoCD EKS Capability ---
# Uses the native EKS ArgoCD Capability (aws_eks_capability)
# EKS provisions and manages ArgoCD outside the cluster, reducing operational overhead.

data "aws_ssoadmin_instances" "this" {}

resource "aws_iam_role" "argocd_capability" {
  name = "${var.cluster_name}-argocd-capability"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "eks.amazonaws.com"
      }
      Action = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "argocd_capability" {
  name = "argocd-capability"
  role = aws_iam_role.argocd_capability.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = "arn:aws:secretsmanager:${var.region}:${data.aws_caller_identity.current.account_id}:secret:temporalci/*"
      }
    ]
  })
}

resource "aws_eks_capability" "argocd" {
  cluster_name              = aws_eks_cluster.this.name
  capability_name           = "argocd"
  type                      = "ARGOCD"
  role_arn                  = aws_iam_role.argocd_capability.arn
  delete_propagation_policy = "RETAIN"

  configuration {
    argo_cd {
      namespace = "argocd"

      aws_idc {
        idc_instance_arn = tolist(data.aws_ssoadmin_instances.this.arns)[0]
        idc_region       = var.region
      }
    }
  }

  tags = {
    Project = "TemporalCI"
  }

  depends_on = [aws_eks_cluster.this]
}

# --- ArgoCD Application for TemporalCI (GitOps self-managing) ---

resource "kubernetes_manifest" "temporalci_app" {
  manifest = {
    apiVersion = "argoproj.io/v1alpha1"
    kind       = "Application"
    metadata = {
      name      = "temporalci"
      namespace = "argocd"
    }
    spec = {
      project = "default"
      source = {
        repoURL        = "https://github.com/AndreKurait/TemporalCI.git"
        targetRevision = "HEAD"
        path           = "deploy/helm"
        helm = {
          valueFiles = ["values-eks.yaml"]
        }
      }
      destination = {
        server    = "https://kubernetes.default.svc"
        namespace = "temporalci"
      }
      syncPolicy = {
        automated = {
          prune    = true
          selfHeal = true
        }
        syncOptions = ["CreateNamespace=true"]
      }
    }
  }

  depends_on = [aws_eks_capability.argocd]
}

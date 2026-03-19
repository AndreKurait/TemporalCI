# --- ArgoCD (Helm-managed, GitOps for TemporalCI) ---

resource "helm_release" "argocd" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  version          = "7.7.16"
  namespace        = "argocd"
  create_namespace = true

  set {
    name  = "server.service.type"
    value = "ClusterIP"
  }

  set {
    name  = "configs.params.server\\.insecure"
    value = "true"
  }

  depends_on = [aws_eks_cluster.this]
}

# --- ArgoCD Application for TemporalCI (self-managing GitOps) ---

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

  depends_on = [helm_release.argocd]
}

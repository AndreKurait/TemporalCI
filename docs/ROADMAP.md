# TemporalCI — Roadmap

## Completed

### Core CI Engine
- [x] GitHub webhook → Temporal workflow → clone → build/test/vet → report
- [x] `.temporalci.yaml` pipeline config with parallel + DAG-based execution
- [x] Per-step timeout, cancel previous runs, cancelled/skipped detection
- [x] Disconnected context for reporting, output truncation, cleanup activity

### K8s Pod Execution
- [x] CI steps run as isolated K8s pods with resource limits
- [x] CI node pool with taints (Auto Mode NodePool CRDs)
- [x] Pod cleanup via scheduled Temporal workflow
- [x] PVC-backed Go module cache + artifact passing between steps

### Observability
- [x] S3 log upload with presigned URLs in PR comments
- [x] Structured logging (slog, JSON, correlation IDs)
- [x] Prometheus metrics (workflow duration, step status, active pods)
- [x] S3 lifecycle (30d → IA, 90d expire)

### GitHub Integration
- [x] GitHub App auth (JWT, installation tokens, PKCS1/PKCS8)
- [x] Check Runs API with per-step annotations and inline output
- [x] PR comments with timing, pass/fail, collapsible logs, Temporal UI link
- [x] OIDC federation (no static AWS credentials)

### Multi-Repo & Self-Service
- [x] Repo registration API (`/api/repos` CRUD)
- [x] Admin dashboard (`/dashboard`)
- [x] Per-repo Slack webhook notifications

### Secrets & Environments
- [x] Secret injection from AWS Secrets Manager
- [x] Environment pipelines with branch-based triggers
- [x] Manual approval gates (Temporal signals + Slack)

### Infrastructure (EKS Auto Mode)
- [x] EKS Auto Mode (no managed node groups)
- [x] ACK for RDS (PostgreSQL 16) and S3
- [x] ArgoCD via EKS Capability (`aws_eks_capability`)
- [x] Secrets Store CSI for automatic rotation
- [x] Terraform manages cluster/IAM/ECR only; ACK manages the rest

### Production Hardening
- [x] Tuned retry policies per activity type
- [x] RBAC — CI pods get zero K8s API permissions
- [x] Network policies isolating CI pods
- [x] Rate limiting (60 req/min/IP) + audit logging
- [x] CloudWatch Observability addon

### Ephemeral Clusters & Helm Testing
- [x] ClusterPool Temporal workflow with warm pool management
- [x] ProvisionCluster activity (EKS Auto Mode via AWS SDK)
- [x] Lease/Release via Temporal signals
- [x] Helm test pipeline workflow (`helm-test` step type)

### Docs & CI/CD
- [x] Astro Starlight docs site with GitHub Pages deployment
- [x] Docker build → ECR → ArgoCD GitOps pipeline
- [x] OIDC bootstrap guide

---

## Remaining

- [ ] Matrix builds — run steps across multiple versions (Go 1.22/1.23, Node 18/20)
- [ ] Monorepo support — detect changed paths, only run affected pipelines
- [ ] Build badges — SVG badge endpoint per repo/branch
- [ ] Webhook replay — re-trigger past CI runs from dashboard
- [ ] Self-hosting — TemporalCI builds and deploys itself on every push

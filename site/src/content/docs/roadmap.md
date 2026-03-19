---
title: Roadmap
description: TemporalCI development roadmap and current status.
---

## Current Status

All planned Q1–Q3 features and most Q4 features are complete. TemporalCI is a fully operational Kubernetes-native CI system running on EKS Auto Mode.

### Completed

- ✅ GitHub webhook → Temporal workflow → clone → build/test/vet → report
- ✅ K8s pod execution with resource limits, tolerations, CI node pool
- ✅ `.temporalci.yaml` pipeline config with parallel + DAG-based execution
- ✅ GitHub App auth with Check Runs API
- ✅ S3 log upload with presigned URLs in PR comments
- ✅ Structured logging (slog) + Prometheus metrics
- ✅ PVC-backed Go module cache + artifact passing
- ✅ Multi-repo registration API + admin dashboard
- ✅ Secret injection from AWS Secrets Manager
- ✅ Environment pipelines with branch triggers + approval gates
- ✅ Slack notifications on pipeline completion
- ✅ EKS Auto Mode with NodePool CRDs
- ✅ ACK for RDS (PostgreSQL 16) and S3
- ✅ ArgoCD via EKS Capability
- ✅ RBAC, network policies, rate limiting, audit logging
- ✅ Secrets Store CSI for automatic rotation
- ✅ Ephemeral EKS cluster pool with warm pool management
- ✅ Helm test pipeline workflow
- ✅ OIDC federation (zero stored credentials)
- ✅ Astro Starlight docs site with GitHub Pages

### Remaining

- Matrix builds — run steps across multiple versions (Go 1.22/1.23, Node 18/20)
- Monorepo support — detect changed paths, only run affected pipelines
- Build badges — SVG badge endpoint per repo/branch
- Webhook replay — re-trigger past CI runs from dashboard
- Self-hosting — TemporalCI builds and deploys itself on every push

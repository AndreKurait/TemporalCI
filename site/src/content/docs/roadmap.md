---
title: Roadmap
description: TemporalCI development roadmap and current status.
---

## Current Status

39 commits, ~1,500 LOC Go, fully operational on EKS.

### What's Working

- ✅ GitHub webhook → Temporal workflow → clone → build/test/vet → report
- ✅ `.temporalci.yaml` pipeline config per-repo
- ✅ Parallel + DAG-based step execution
- ✅ Per-step timeout, cancel previous runs on new push
- ✅ PR comments with per-step timing, collapsible logs
- ✅ Direct link to Temporal Web UI in PR comments
- ✅ EKS cluster with managed node groups, Pod Identity
- ✅ ArgoCD auto-sync from Git
- ✅ Docker build via GitHub Actions → ECR → ArgoCD deploys
- ✅ OIDC federation (zero stored credentials)

## Q1: Production-Grade Execution

**Goal**: CI steps run as isolated K8s pods with proper resource limits, caching, and log persistence.

- K8s pod execution in production (code exists, needs wiring)
- Dedicated CI node pool with taints
- S3 log upload with presigned URLs
- Structured logging with correlation IDs
- Prometheus metrics
- PVC-backed Go module and Docker layer cache

## Q2: Multi-Repo & GitHub App

**Goal**: Any team can onboard their repo. GitHub App replaces PAT.

- GitHub App with installation-scoped tokens
- Check Runs API with per-test annotations
- JUnit XML parsing
- Repo registration API
- Secret injection into CI steps
- Environment pipelines with approval gates

## Q3: Infrastructure Modernization

**Goal**: EKS Auto Mode, RDS via ACK, production hardening.

- EKS Auto Mode (default for EKS deployments)
- ACK for RDS and S3
- ArgoCD EKS Capability
- RBAC and network policies
- Secret rotation
- S3 lifecycle policies

## Q4: Ephemeral Clusters & Advanced Features

**Goal**: On-demand EKS clusters for Helm chart testing. Matrix builds.

- Cluster pool manager (warm pool of EKS clusters)
- `helm-test` step type
- Matrix builds (multiple Go/Node versions)
- Monorepo support (changed-path detection)
- Build badges
- Self-hosting milestone (TemporalCI builds itself)

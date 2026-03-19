---
title: System Overview
description: TemporalCI architecture — how webhooks, Temporal, and Kubernetes work together.
---

## High-Level Architecture

```
GitHub webhook → Webhook Server → Temporal Server → Worker → K8s Pods
                                                          → S3 (logs)
                                                          → GitHub API (results)
```

| Component | What It Does |
|-----------|-------------|
| **Webhook Server** | Validates GitHub signatures, starts Temporal workflows |
| **Temporal Server** | Durable workflow orchestration, task queues, history |
| **Worker** | Executes CI activities — clone, run steps, report results |
| **CI Job Pods** | Ephemeral K8s pods running each CI step in isolation |

## Data Flow

1. GitHub sends a push/PR webhook event
2. Webhook server validates the HMAC signature
3. Webhook server starts a `CIPipeline` Temporal workflow
4. Worker picks up the workflow from the task queue
5. Worker clones the repo, reads `.temporalci.yaml`
6. Worker creates K8s pods for each step (respecting DAG dependencies)
7. Worker streams pod logs and collects exit codes
8. Worker uploads full logs to S3 with presigned URLs
9. Worker reports results to GitHub (commit status + PR comment)

## Security Model

| Layer | Mechanism |
|-------|-----------|
| **Webhook validation** | HMAC-SHA256 signature verification |
| **Secret storage** | K8s Secrets (local) or AWS Secrets Manager (prod) |
| **IAM** | EKS Pod Identity — least-privilege per component |
| **Network isolation** | CI jobs on dedicated `ci-jobs` NodePool with taints |
| **Container isolation** | Each CI step in its own ephemeral pod |

## Project Structure

```
cmd/
  worker/          # Temporal worker entrypoint
  webhook/         # GitHub webhook HTTP server
internal/
  workflows/       # CIPipeline, ClusterPool, HelmTestPipeline
  activities/      # CloneRepo, RunStep, ReportResults, UploadLog
  k8s/             # Pod creation, log streaming, cleanup
  config/          # App config + .temporalci.yaml parser
deploy/
  helm/            # Umbrella Helm chart (Temporal + PostgreSQL subcharts)
  terraform/       # EKS, ECR, IAM, OIDC provider
```

## Key Design Decisions

- **Temporal for orchestration** — workflows are plain Go functions with full replay semantics
- **K8s pods for isolation** — each CI step gets its own pod, image, and resource limits
- **EKS Auto Mode for compute** — nodes provisioned on demand, no idle capacity
- **ArgoCD for deployment** — push to main → ECR → ArgoCD syncs
- **OIDC for auth** — GitHub Actions → AWS IAM, no stored credentials

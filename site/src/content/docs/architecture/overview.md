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
| **CI Job Pods** | Ephemeral K8s pods running build steps; not a complete tenant isolation boundary |

## Data Flow

1. GitHub sends a push/PR webhook event
2. Webhook server validates the HMAC signature
3. Webhook server starts a `CIPipeline` Temporal workflow
4. Worker picks up the workflow from the task queue
5. Worker clones the repo and selects matching named pipelines from `.temporalci/*.yaml`
6. Worker creates K8s pods for each step; regular steps run in declaration order and check dependencies
7. Worker streams pod logs and collects exit codes
8. Worker uploads logs when object storage is configured
9. Worker reports results when GitHub credentials and event metadata permit it

## Security Model

| Layer | Mechanism |
|-------|-----------|
| **Webhook validation** | HMAC-SHA256 signature verification |
| **Secret storage** | K8s Secrets in the local/EKS values; optional AWS Secrets Manager integration |
| **IAM** | EKS role and Pod Identity examples require configuration and permission review |
| **Scheduling** | Optional `ci-jobs` NodePool; taints control placement, not network access |
| **Network policy** | Disabled by default; enforcement depends on the cluster CNI |
| **Execution** | Ephemeral K8s pods; local fallback runs commands as the worker's operating-system user |

Temporal records workflow decisions, not worker-local files or running commands. Repository registration is file-backed; sessions and notifications are in memory. See the [security model](/TemporalCI/concepts/security/) before adding replicas or accepting untrusted code.

## Project Structure

```
cmd/
  worker/          # Temporal worker entrypoint
  webhook/         # GitHub webhook HTTP server
internal/
  workflows/       # CIPipeline, ClusterPool, HelmTestPipeline
  activities/      # CloneRepo, RunStep, ReportResults, UploadLog
  k8s/             # Pod creation, log streaming, cleanup
  config/          # App config + named .temporalci/*.yaml pipeline parser
deploy/
  helm/            # Umbrella Helm chart (Temporal + PostgreSQL subcharts)
  terraform/       # EKS, ECR, IAM, OIDC provider
```

## Key Design Decisions

- **Temporal for orchestration** — workflows are plain Go functions with full replay semantics
- **K8s execution** — each Kubernetes step gets its own pod, image, and resource limits
- **EKS Auto Mode for compute** — optional node provisioning for the EKS experiment
- **ArgoCD deployment example** — an operator-configured controller can apply chart/value changes; repository CI does not publish images or update GitOps tags
- **Repository automation** — code checks and GitHub Pages deployment do not assume AWS roles; runtime GitHub/OAuth secrets and credentials in workflow history still require protection

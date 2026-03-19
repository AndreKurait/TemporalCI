---
title: Environment Variables
description: Environment variables used by TemporalCI components.
---

## Worker

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server gRPC address |
| `TEMPORAL_NAMESPACE` | `default` | Temporal namespace |
| `TEMPORAL_TASK_QUEUE` | `temporalci` | Task queue name |
| `TEMPORALCI_K8S_ENABLED` | `false` | Enable K8s pod execution (vs local shell) |
| `GITHUB_TOKEN` | — | GitHub API token (or mounted from secret) |
| `LOG_BUCKET` | — | S3 bucket for CI logs (production) |
| `AWS_REGION` | — | AWS region for S3 operations |

## Webhook Server

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_PORT` | `8080` | HTTP listen port |
| `WEBHOOK_SECRET` | — | GitHub webhook HMAC secret (or mounted from secret) |
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server gRPC address |
| `TEMPORAL_NAMESPACE` | `default` | Temporal namespace |
| `TEMPORAL_TASK_QUEUE` | `temporalci` | Task queue name |

## Secret File Mounts

In both local and production modes, secrets can be mounted as files:

| Path | Content | Source (Local) | Source (Production) |
|------|---------|----------------|---------------------|
| `/secrets/github-webhook-secret` | Webhook HMAC secret | K8s Secret | Secrets Manager via CSI |
| `/secrets/github-token` | GitHub API token | K8s Secret | Secrets Manager via CSI |

The application checks for file mounts first, then falls back to environment variables.

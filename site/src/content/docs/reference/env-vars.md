---
banner:
  content: 'Archived proof of concept — November 2026. This documentation is a frozen snapshot; development and GitHub automation have ended.'
title: Environment Variables
description: Environment variables used by TemporalCI components.
---

## Worker

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST_PORT` | `localhost:7233` | Temporal server gRPC address |
| `METRICS_HOST` | `127.0.0.1` | Metrics/health listener host on port 9090; Helm binds inside the pod |
| `GITHUB_TOKEN` | — | GitHub API token (or mounted from secret) |
| `LOG_BUCKET` | — | S3 bucket for CI logs (production) |
| `AWS_REGION` | `us-east-1` | AWS client region |
| `SECRETS_PREFIX` | — | Prefix for workflow secret lookups |
| `PRIVILEGED_REPOS` | — | Comma-separated repository allowlist for privileged execution and Docker-in-Docker; empty denies both |

The namespace is currently `default` and the task queue is `temporalci-task-queue`; there are no environment overrides for those values. The worker attempts in-cluster Kubernetes configuration when a service-account token is present. Otherwise it can run commands on the worker host, so use only trusted pipeline definitions.

## Webhook Server

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_HOST` | `127.0.0.1` | HTTP listen host; Helm binds inside the pod behind a ClusterIP service |
| `PORT` | `8080` | HTTP listen port |
| `GITHUB_WEBHOOK_SECRET` | — | GitHub webhook HMAC secret; missing configuration rejects requests |
| `TEMPORAL_HOST_PORT` | `localhost:7233` | Temporal server gRPC address |
| `GITHUB_CLIENT_ID` | — | GitHub OAuth client ID |
| `GITHUB_CLIENT_SECRET` | — | GitHub OAuth client secret |
| `GITHUB_ALLOWED_USERS` | — | Comma-separated operator logins; empty means no operator access |
| `SESSION_COOKIE_SECURE` | `true` | Set `false` only for isolated loopback HTTP development |
| `PUBLIC_READ` | `false` | Opt in to anonymous build metadata, parameters, and log-link access |
| `REPO_STORE_PATH` | `/data/repos.json` | Repository-registration file |

## Secret File Mounts

In both local and production modes, secrets can be mounted as files:

| Path | Content | Source (Local) | Source (Production) |
|------|---------|----------------|---------------------|
| `/etc/temporalci/github-webhook-secret` | Webhook HMAC secret | K8s Secret | Secrets Manager via CSI |
| `/etc/temporalci/github-token` | GitHub API token | K8s Secret | Secrets Manager via CSI |

The application checks the environment variable first, then falls back to the file. OAuth credentials use environment variables; the Helm chart reads the client secret from a Kubernetes Secret.

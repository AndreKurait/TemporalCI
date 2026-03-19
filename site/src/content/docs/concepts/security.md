---
title: Security Model
description: How TemporalCI secures CI execution, secrets, and infrastructure access.
---

## Principles

1. **Zero stored credentials** — OIDC federation for AWS, no long-lived keys
2. **Least privilege** — each component gets only the permissions it needs
3. **Isolation** — CI pods can't access the cluster or other workloads
4. **Defense in depth** — RBAC + network policies + pod security

## Authentication Layers

### GitHub → AWS (OIDC)

GitHub Actions authenticates to AWS using OpenID Connect federation. No AWS access keys are stored anywhere:

```
GitHub Actions → OIDC token → AWS STS AssumeRoleWithWebIdentity → temporary credentials
```

The IAM trust policy restricts access to the specific GitHub repo.

### Pods → AWS (Pod Identity)

In production, pods authenticate to AWS via [EKS Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html):

| ServiceAccount | IAM Permissions | Used By |
|----------------|----------------|---------|
| `temporalci-worker` | ECR pull/push, S3 read/write | Worker pods |
| `temporalci-ci-job` | S3 write (logs only) | CI job pods |
| `temporalci-webhook` | Minimal (no AWS access) | Webhook server |

### Webhook Validation

Every GitHub webhook is validated using HMAC-SHA256 signature verification before any workflow is started.

## CI Pod Isolation

CI pods are untrusted by default — they run arbitrary user code.

### RBAC

The `temporalci-ci-job` ServiceAccount has **zero** Kubernetes API permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ci-job-minimal
rules: []  # No permissions
```

CI pods cannot list pods, read secrets, or access any K8s API.

### Network Policies

CI pods are network-isolated:

- **No inbound traffic** — nothing can connect to CI pods
- **Egress: DNS allowed** — pods can resolve hostnames
- **Egress: HTTPS allowed** — pods can pull dependencies from the internet
- **Egress: cluster-internal blocked** — 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 are denied

This prevents a compromised CI pod from attacking Temporal, the K8s API, or other cluster services.

## Secrets Management

### Production

Secrets are stored in AWS Secrets Manager and mounted into pods via the Secrets Store CSI Driver:

| Secret | Used By |
|--------|---------|
| `temporalci/github-webhook-secret` | Webhook server — HMAC validation |
| `temporalci/github-token` | Worker — GitHub API access |

RDS credentials are managed by ACK and stored as K8s Secrets automatically.

### Local

Same file mount paths using plain K8s Secrets. Application code is identical in both modes.

## Rate Limiting

The webhook endpoint is rate-limited to 60 requests per minute per IP to prevent abuse. All webhook and API requests are audit-logged.

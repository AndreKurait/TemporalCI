---
title: Security Model
description: Security boundaries and limitations of the TemporalCI proof of concept.
---

## Principles

TemporalCI is a POC for trusted repositories and disposable environments. It is not a hardened multi-tenant runner. Review effective permissions, network policy enforcement, and secret handling before deploying an experiment.

1. **Explicit operators** — GitHub OAuth plus an allowlist gates management.
2. **Private defaults** — native listeners use loopback and Helm uses ClusterIP services by default.
3. **Scoped credentials** — choose narrowly scoped permissions for each experiment.
4. **Known limits** — workflow history, workspace persistence, and pod isolation need further hardening.

## Authentication Layers

### GitHub → AWS (OIDC)

The deployment workflow can authenticate to AWS using OpenID Connect federation, avoiding a long-lived AWS key for that specific integration:

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

GitHub webhooks require HMAC-SHA256 signature verification before any workflow is started. A missing webhook secret rejects requests.

### Browser and Management Access

GitHub OAuth requires `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and `GITHUB_ALLOWED_USERS`. The last variable is a comma-separated operator allowlist; an empty list grants no access. OAuth state is browser-bound, expires after ten minutes, and can be used once.

Management APIs and custom triggers require an allowed operator. Build reads are private by default. `PUBLIC_READ=true` explicitly exposes build metadata, parameters, and log links. Public badges expose status independently. Cookies are secure by default; the loopback HTTP development opt-out is `SESSION_COOKIE_SECURE=false`.

Sessions live in memory and disappear on restart. The POC does not implement per-repository roles or shared session storage for multiple replicas.

## CI Pod Isolation

Pipeline commands execute user-supplied code. Run only repositories you trust. A worker outside Kubernetes can execute commands directly on its host.

### RBAC

The `temporalci-ci-job` ServiceAccount has **zero** Kubernetes API permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ci-job-minimal
rules: []  # No permissions
```

Verify the service account's effective bindings in your cluster; the chart alone cannot establish a tenant isolation guarantee.

### Network Policies

The chart includes an optional network policy, disabled by default. When enabled and enforced by the cluster's CNI, it is intended to provide:

- **No inbound traffic** — nothing can connect to CI pods
- **Egress: DNS allowed** — pods can resolve hostnames
- **Egress: HTTPS allowed** — pods can pull dependencies from the internet
- **Egress: cluster-internal blocked** — 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 are denied

Review the rendered rules and your network topology before relying on this policy. Keep Temporal, its database, and other control services private.

## Secrets Management

### Production

Secrets are stored in AWS Secrets Manager and mounted into pods via the Secrets Store CSI Driver:

| Secret | Used By |
|--------|---------|
| `temporalci/github-webhook-secret` | Webhook server — HMAC validation |
| `temporalci/github-token` | Worker — GitHub API access |

RDS credentials are managed by ACK and stored as K8s Secrets automatically.

Fetched secrets and assumed-role credentials can appear in activity arguments or results and therefore in Temporal history. Log masking does not remove them from history. Do not use production credentials with this POC; protecting or redesigning credential delivery requires additional work.

### Local

Same file mount paths using plain K8s Secrets. Application code is identical in both modes.

## Rate Limiting

The webhook endpoint is rate-limited to 60 requests per minute per IP to prevent abuse. All webhook and API requests are audit-logged.

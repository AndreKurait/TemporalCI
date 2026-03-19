---
title: GitHub Secrets
description: All secrets required for TemporalCI CI/CD and production deployment.
---

## CI/CD Secrets (GitHub Actions)

Used by GitHub Actions workflows to build and push Docker images.

| Variable | Used By | Description |
|----------|---------|-------------|
| `AWS_ROLE_ARN` | Docker Build, Terraform | IAM role ARN for OIDC federation |
| `AWS_REGION` | Docker Build, Terraform | AWS region (e.g., `us-east-1`) |
| `ECR_REGISTRY` | Docker Build | ECR registry URL |

:::note
TemporalCI uses OIDC federation — no AWS access keys are stored in GitHub. See [OIDC Bootstrap](/TemporalCI/production/oidc-bootstrap/) for setup.
:::

## Terraform Secrets

Additional secrets for the Terraform Apply workflow.

| Secret | Used By | Description |
|--------|---------|-------------|
| `VPC_ID` | Terraform Apply | VPC ID for the EKS cluster |
| `PRIVATE_SUBNET_IDS` | Terraform Apply | JSON list of subnet IDs |

## Production Runtime Secrets

Stored in AWS Secrets Manager (not GitHub) and mounted into pods at runtime.

| Secret Path | Used By | Description |
|-------------|---------|-------------|
| `${SECRETS_PREFIX}/github-webhook-secret` | Webhook server | HMAC secret for webhook validation |
| `${SECRETS_PREFIX}/github-token` | Worker | GitHub token for Check Runs and PR comments |

## Setting Secrets

### GitHub Variables (via CLI)

```bash
gh variable set AWS_ROLE_ARN --body "arn:aws:iam::123456789:role/temporalci-github-actions" --repo AndreKurait/TemporalCI
gh variable set AWS_REGION --body "us-east-1" --repo AndreKurait/TemporalCI
gh variable set ECR_REGISTRY --body "123456789.dkr.ecr.us-east-1.amazonaws.com" --repo AndreKurait/TemporalCI
```

### AWS Secrets Manager

```bash
aws secretsmanager create-secret \
  --name "temporalci/github-webhook-secret" \
  --secret-string "<your-webhook-secret>"

aws secretsmanager create-secret \
  --name "temporalci/github-token" \
  --secret-string "ghp_<your-token>"
```

---
title: OIDC Bootstrap
description: One-time setup for GitHub Actions OIDC federation with AWS.
---

## How It Works

GitHub Actions authenticates to AWS using OIDC (OpenID Connect) federation. No AWS access keys are stored anywhere.

```
GitHub Actions → requests OIDC token from GitHub IdP
              → presents token to AWS STS AssumeRoleWithWebIdentity
              → AWS validates token against IAM OIDC provider
              → returns temporary credentials (~15 min)
```

## One-Time Bootstrap

The OIDC provider and IAM role must exist before GitHub Actions can use them. This requires a one-time `terraform apply` with temporary credentials:

```bash
cd deploy/terraform

# Get temporary credentials (any method)
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_SESSION_TOKEN=...

terraform init
terraform apply
```

This creates:
- `aws_iam_openid_connect_provider.github` — trusts `token.actions.githubusercontent.com`
- `aws_iam_role.github_actions` — assumable only by `repo:AndreKurait/TemporalCI:*`

## After Bootstrap

All GitHub Actions workflows use OIDC automatically. No credentials to rotate.

Set the role ARN as a GitHub repository variable:

```bash
gh variable set AWS_ROLE_ARN \
  --body "arn:aws:iam::123456789:role/temporalci-github-actions" \
  --repo AndreKurait/TemporalCI
```

## Security

- IAM role trust policy restricts to this specific GitHub repo
- Least-privilege permissions (ECR push for CI, broader for Terraform)
- OIDC tokens are ephemeral (~15 minutes)
- No long-lived credentials stored anywhere

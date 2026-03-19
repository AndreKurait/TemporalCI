# GitHub Actions OIDC Bootstrap

## How It Works

GitHub Actions authenticates to AWS using OIDC (OpenID Connect) federation.
No AWS access keys are stored anywhere — GitHub requests a short-lived token
from its own identity provider, and AWS IAM trusts it directly.

```
GitHub Actions → requests OIDC token from GitHub IdP
                → presents token to AWS STS AssumeRoleWithWebIdentity
                → AWS validates token against IAM OIDC provider
                → returns temporary credentials (15 min)
```

## One-Time Bootstrap

The OIDC provider and IAM role must exist in AWS before GitHub Actions can use them.
This requires a one-time `terraform apply` with temporary credentials:

```bash
cd deploy/terraform

# Get temporary credentials for account <ACCOUNT_ID> (any method)
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

The role ARN is set as a GitHub repository variable:
```
AWS_ROLE_ARN = arn:aws:iam::<ACCOUNT_ID>:role/temporalci-github-actions
```

## Security

- The IAM role trust policy restricts to this specific GitHub repo
- The role has least-privilege permissions (ECR push only for CI, full for Terraform)
- OIDC tokens are ephemeral (valid ~15 minutes)
- No long-lived credentials stored anywhere

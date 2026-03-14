# GitHub Secrets

All secrets required for TemporalCI CI/CD pipelines and production deployment.

## Required for CI/CD

These secrets are used by GitHub Actions workflows to build and push Docker images.

| Secret | Used By | Description |
|--------|---------|-------------|
| `AWS_ACCESS_KEY_ID` | Docker Build, Terraform | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | Docker Build, Terraform | AWS secret key |
| `AWS_SESSION_TOKEN` | Docker Build, Terraform | AWS session token (if using temporary credentials) |
| `AWS_REGION` | Docker Build, Terraform | AWS region (e.g., `us-east-1`) |
| `ECR_REGISTRY` | Docker Build | ECR registry URL (e.g., `123456789.dkr.ecr.us-east-1.amazonaws.com`) |

## Required for Terraform

These additional secrets are needed when using the Terraform Apply workflow to bootstrap infrastructure.

| Secret | Used By | Description |
|--------|---------|-------------|
| `VPC_ID` | Terraform Apply | VPC ID for the EKS cluster |
| `PRIVATE_SUBNET_IDS` | Terraform Apply | JSON list of subnet IDs (e.g., `["subnet-aaa","subnet-bbb"]`) |

## Required for Production Runtime

These are stored in AWS Secrets Manager (not GitHub) and mounted into pods at runtime.

| Secret Path | Used By | Description |
|-------------|---------|-------------|
| `${SECRETS_PREFIX}/github-webhook-secret` | Webhook server | HMAC secret for validating GitHub webhook signatures |
| `${SECRETS_PREFIX}/github-token` | Worker | GitHub token for creating Check Runs and PR comments |

## Setting Secrets

### GitHub Secrets (via CLI)

```bash
gh secret set AWS_ACCESS_KEY_ID --repo AndreKurait/TemporalCI
gh secret set AWS_SECRET_ACCESS_KEY --repo AndreKurait/TemporalCI
gh secret set AWS_REGION --body "us-east-1" --repo AndreKurait/TemporalCI
gh secret set ECR_REGISTRY --body "123456789.dkr.ecr.us-east-1.amazonaws.com" --repo AndreKurait/TemporalCI
```

### AWS Secrets Manager

```bash
aws secretsmanager create-secret \
  --name "temporalci/github-webhook-secret" \
  --secret-string "your-webhook-secret"

aws secretsmanager create-secret \
  --name "temporalci/github-token" \
  --secret-string "ghp_your_token"
```

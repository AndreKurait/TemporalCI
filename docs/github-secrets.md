# GitHub Secrets

Required GitHub Secrets for TemporalCI deployment.

## AWS Credentials (for CI/CD)

| Secret | Description |
|--------|-------------|
| `AWS_ACCESS_KEY_ID` | AWS access key for CI operations |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key for CI operations |
| `AWS_REGION` | AWS region (e.g., `us-east-1`) |

## Resource Identifiers

| Secret | Description | Example |
|--------|-------------|---------|
| `ECR_REGISTRY` | ECR registry URL | `123456789.dkr.ecr.us-east-1.amazonaws.com` |
| `LOG_BUCKET` | S3 bucket for CI logs | `my-temporalci-logs` |
| `SECRETS_PREFIX` | AWS Secrets Manager prefix | `temporalci` |

## GitHub App (future)

| Secret | Description |
|--------|-------------|
| `GITHUB_APP_ID` | GitHub App ID for PR checks |
| `GITHUB_APP_PRIVATE_KEY` | GitHub App private key |
| `GITHUB_WEBHOOK_SECRET` | Webhook signature validation secret |

# Production Deployment (EKS Auto Mode)

This guide walks through deploying TemporalCI on AWS EKS with Auto Mode.

## Prerequisites

- AWS CLI configured with admin access
- Terraform ≥ 1.9
- kubectl
- Helm 3
- A VPC with private subnets

## 1. Bootstrap Infrastructure

```bash
cd deploy/terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your values:

```hcl
region             = "us-east-1"
vpc_id             = "vpc-0abc123..."
private_subnet_ids = ["subnet-0aaa...", "subnet-0bbb..."]
```

Apply:

```bash
terraform init
terraform plan
terraform apply
```

This creates:
- EKS Auto Mode cluster with `general-purpose` node pool
- ECR repository for container images
- IAM roles for the cluster, nodes, and Pod Identity
- Pod Identity associations for worker, webhook, and CI job service accounts
- EKS add-ons: Secrets Store CSI Driver, CloudWatch Observability

Alternatively, use the GitHub Actions workflow:

1. Set `VPC_ID` and `PRIVATE_SUBNET_IDS` as GitHub Secrets
2. Go to Actions → "Terraform Apply" → Run workflow → select "apply"

## 2. Configure kubectl

```bash
aws eks update-kubeconfig --name temporalci --region us-east-1
```

## 3. Seed Secrets

```bash
SECRETS_PREFIX=temporalci

aws secretsmanager create-secret \
  --name "${SECRETS_PREFIX}/github-webhook-secret" \
  --secret-string "your-webhook-secret"

aws secretsmanager create-secret \
  --name "${SECRETS_PREFIX}/github-token" \
  --secret-string "ghp_your_github_token"
```

## 4. Deploy with Helm

```bash
# Get ECR registry URL from Terraform output
ECR_REGISTRY=$(terraform -chdir=deploy/terraform output -raw ecr_repository_url | cut -d/ -f1)

helm install temporalci ./deploy/helm \
  --namespace temporalci --create-namespace \
  --set image.registry="${ECR_REGISTRY}" \
  --set image.tag=latest \
  --set aws.region=us-east-1 \
  --set logs.s3Bucket=your-log-bucket \
  --set secrets.aws.enabled=true \
  --set secrets.prefix=temporalci \
  --set rds.enabled=true \
  --set s3.enabled=true \
  --set nodePool.system.enabled=true \
  --set nodePool.ciJobs.enabled=true \
  --set serviceAccounts.worker.roleArn=arn:aws:iam::YOUR_ACCOUNT:role/temporalci-worker \
  --set serviceAccounts.webhook.roleArn=arn:aws:iam::YOUR_ACCOUNT:role/temporalci-webhook \
  --set serviceAccounts.ciJob.roleArn=arn:aws:iam::YOUR_ACCOUNT:role/temporalci-ci-job
```

## 5. Set Up GitHub Webhook

1. Go to your repo → Settings → Webhooks → Add webhook
2. **Payload URL:** `https://your-webhook-endpoint/webhook`
3. **Content type:** `application/json`
4. **Secret:** same value as `github-webhook-secret` above
5. **Events:** select "Pushes" and "Pull requests"

## 6. Verify

```bash
# Check pods are running
kubectl get pods -n temporalci

# Check Temporal Web UI
kubectl port-forward svc/temporalci-temporal-web -n temporalci 8088:8088

# Trigger a test by pushing to a configured repo
```

## GitOps with Argo CD

For automated deployments, configure the Argo CD EKS Capability to sync from this repo:

1. Enable Argo CD capability on your EKS cluster (via Terraform or console)
2. Create an Argo CD Application pointing at `deploy/helm/`
3. Configure Helm value overrides with your production values
4. Merging to `main` automatically deploys via Argo CD sync

## Updating

Push to `main` triggers the Docker Build workflow, which pushes a new image to ECR. If using Argo CD, it detects the change and syncs automatically. Otherwise:

```bash
helm upgrade temporalci ./deploy/helm --namespace temporalci --reuse-values \
  --set image.tag=$(git rev-parse HEAD)
```

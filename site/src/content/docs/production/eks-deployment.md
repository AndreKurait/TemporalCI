---
title: EKS Deployment
description: Deploy TemporalCI to production on AWS EKS.
---

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

Edit `terraform.tfvars`:

```hcl
region             = "us-east-1"
vpc_id             = "vpc-0abc123..."
private_subnet_ids = ["subnet-0aaa...", "subnet-0bbb..."]
```

```bash
terraform init
terraform apply
```

This creates:
- EKS Auto Mode cluster with `general-purpose` node pool
- ECR repository for container images
- IAM roles for the cluster, nodes, and Pod Identity
- Pod Identity associations for worker, webhook, and CI job service accounts
- EKS add-ons: Secrets Store CSI Driver, CloudWatch Observability

## 2. Configure kubectl

```bash
aws eks update-kubeconfig --name temporalci --region us-east-1
```

## 3. Seed Secrets

```bash
aws secretsmanager create-secret \
  --name "temporalci/github-webhook-secret" \
  --secret-string "<your-webhook-secret>"

aws secretsmanager create-secret \
  --name "temporalci/github-token" \
  --secret-string "ghp_<your-token>"
```

## 4. Deploy with Helm

```bash
ECR_REGISTRY=$(terraform -chdir=deploy/terraform output -raw ecr_repository_url | cut -d/ -f1)

helm install temporalci ./deploy/helm \
  --namespace temporalci --create-namespace \
  --set image.registry="${ECR_REGISTRY}" \
  --set image.tag=latest \
  --set aws.region=us-east-1 \
  --set secrets.aws.enabled=true \
  --set secrets.prefix=temporalci
```

## 5. Set Up GitHub Webhook

1. Go to your repo → Settings → Webhooks → Add webhook
2. **Payload URL:** `https://your-webhook-endpoint/webhook`
3. **Content type:** `application/json`
4. **Secret:** same value as `github-webhook-secret`
5. **Events:** Pushes and Pull requests

## 6. Verify

```bash
kubectl get pods -n temporalci
kubectl port-forward svc/temporalci-temporal-web -n temporalci 8088:8088
```

## GitOps with Argo CD

For automated deployments:

1. Enable Argo CD capability on your EKS cluster
2. Create an Argo CD Application pointing at `deploy/helm/`
3. Configure Helm value overrides with production values
4. Merging to `main` automatically deploys via Argo CD sync

## Updating

Push to `main` triggers the Docker Build workflow → ECR → ArgoCD syncs automatically. Manual upgrade:

```bash
helm upgrade temporalci ./deploy/helm --namespace temporalci --reuse-values \
  --set image.tag=$(git rev-parse HEAD)
```

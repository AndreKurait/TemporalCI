# EKS Auto Mode Migration Runbook

## Overview
Migrate from managed node groups to EKS Auto Mode with zero downtime using blue/green cluster strategy.

## Prerequisites
- Terraform state backed up
- ArgoCD Application paused
- All CI workflows drained (no active runs)

## Steps

### 1. Provision New Auto Mode Cluster
```bash
# Create new cluster alongside existing one
export NEW_CLUSTER="temporalci-v2"
cd deploy/terraform
terraform workspace new automode
terraform apply -var="cluster_name=$NEW_CLUSTER"
```

### 2. Deploy TemporalCI to New Cluster
```bash
aws eks update-kubeconfig --name $NEW_CLUSTER --region us-east-1
helm install temporalci deploy/helm \
  -n temporalci --create-namespace \
  -f deploy/helm/values.yaml \
  -f deploy/helm/values-eks.yaml \
  --set autoMode.enabled=true \
  --set ack.rds.enabled=true \
  --set networkPolicy.enabled=true
```

### 3. Verify New Cluster
```bash
# Check NodePools created
kubectl get nodepools
# Check pods running
kubectl get pods -n temporalci
# Test webhook endpoint
curl -X POST http://<new-webhook-lb>/health
```

### 4. Switch DNS / Webhook URL
```bash
# Update GitHub webhook URL to point to new cluster's LB
# Update ArgoCD Application to target new cluster
kubectl apply -f deploy/argocd/application.yaml
```

### 5. Drain Old Cluster
```bash
# Wait for all active workflows to complete
# Scale down old cluster workers to 0
# Verify no traffic on old LB
```

### 6. Decommission Old Cluster
```bash
cd deploy/terraform
terraform workspace select default
terraform destroy
terraform workspace select automode
# Rename workspace to default for ongoing use
```

## Rollback
If issues arise, revert DNS/webhook URL to old cluster. Old cluster remains running until explicitly destroyed.

## Validation Checklist
- [ ] NodePools `system` and `ci-jobs` created
- [ ] Temporal server healthy
- [ ] Webhook receiving events
- [ ] CI pods scheduling on `ci-jobs` NodePool
- [ ] S3 log upload working
- [ ] GitHub Check Runs appearing
- [ ] Network policies enforced (CI pods can't reach cluster services)

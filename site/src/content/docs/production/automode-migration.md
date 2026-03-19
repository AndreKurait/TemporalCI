---
title: Auto Mode Migration
description: Migrate from managed node groups to EKS Auto Mode with zero downtime.
---

## Overview

Blue/green cluster migration from managed node groups to EKS Auto Mode.

## Prerequisites

- Terraform state backed up
- ArgoCD Application paused
- All CI workflows drained (no active runs)

## Steps

### 1. Provision New Auto Mode Cluster

```bash
export NEW_CLUSTER="temporalci-v2"
cd deploy/terraform
terraform workspace new automode
terraform apply -var="cluster_name=$NEW_CLUSTER"
```

### 2. Deploy TemporalCI

```bash
aws eks update-kubeconfig --name $NEW_CLUSTER --region us-east-1
helm install temporalci deploy/helm \
  -n temporalci --create-namespace \
  -f deploy/helm/values.yaml \
  --set autoMode.enabled=true
```

### 3. Verify

```bash
kubectl get nodepools
kubectl get pods -n temporalci
curl -X POST http://<new-webhook-lb>/health
```

### 4. Switch Traffic

Update GitHub webhook URL to point to the new cluster's load balancer.

### 5. Decommission Old Cluster

```bash
cd deploy/terraform
terraform workspace select default
terraform destroy
```

## Rollback

Revert the webhook URL to the old cluster. It remains running until explicitly destroyed.

## Validation Checklist

- [ ] NodePools `system` and `ci-jobs` created
- [ ] Temporal server healthy
- [ ] Webhook receiving events
- [ ] CI pods scheduling on `ci-jobs` NodePool
- [ ] S3 log upload working
- [ ] GitHub Check Runs appearing

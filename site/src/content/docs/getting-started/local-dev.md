---
title: Local Development
description: Run TemporalCI locally with minikube for development and testing.
---

## Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/)
- [Helm 3](https://helm.sh/docs/intro/install/)
- kubectl

## Start the Cluster

```bash
minikube start
```

## Install TemporalCI

```bash
helm install temporalci ./deploy/helm -f deploy/helm/values-local.yaml
```

## Create Secrets

```bash
kubectl create secret generic temporalci-secrets \
  --from-literal=github-webhook-secret=dev-secret \
  --from-literal=github-token=ghp_...
```

## Access the Temporal Web UI

```bash
kubectl port-forward svc/temporalci-temporal-web 8088:8088
```

Open [http://localhost:8088](http://localhost:8088) to view workflows.

## Trigger a Test Run

```bash
# Port-forward the webhook server
kubectl port-forward svc/temporalci-webhook 8080:8080

# Send a test webhook event
./scripts/local-trigger.sh owner/repo main
```

## Local vs Production

| | Local (minikube) | Production (EKS) |
|---|---|---|
| **Temporal DB** | PostgreSQL subchart | RDS via ACK |
| **Secrets** | K8s Secrets | AWS Secrets Manager |
| **Logging** | stdout / kubectl logs | CloudWatch + S3 |
| **Compute** | Single node | EKS Auto Mode |
| **IAM** | N/A | EKS Pod Identity |

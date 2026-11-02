---
title: Local Development
description: Run TemporalCI locally with minikube for development and testing.
---

## Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/)
- [Helm 3](https://helm.sh/docs/intro/install/)
- kubectl
- Docker

This is a POC stack with Temporal, PostgreSQL, LocalStack, and a CI worker. Use a disposable local cluster and trusted repositories. For code-only development, `make check` needs none of these services.

## Build and Start the Cluster

```bash
./scripts/local-up.sh
```

The script starts minikube if needed, enables its registry, builds and loads the worker/webhook image, fetches Helm dependencies, and installs into the `temporalci` namespace. Its default new cluster requests 8 CPUs and 16 GiB of memory. Start minikube yourself with suitable resources first if you need different settings.

The script creates random database and webhook credentials in Kubernetes Secrets when they do not already exist, preserving them across upgrades. The local values keep the services inside the cluster. Dependency or readiness failures stop the script; inspect `kubectl get pods -n temporalci` before retrying.

## Access the Temporal Web UI

```bash
kubectl port-forward svc/temporalci-web -n temporalci 8088:8080
```

Open [http://localhost:8088](http://localhost:8088) to view workflows.

## Trigger a Test Run

```bash
# Port-forward the webhook server
kubectl port-forward svc/temporalci-webhook -n temporalci 8080:8080

# Send a test webhook event
./scripts/local-trigger.sh owner/repo main
```

The helper reads the local webhook Secret through kubectl and signs its request. Set `WEBHOOK_SECRET` explicitly when testing a different instance. It clones and executes the chosen repository; use one you control. Missing GitHub credentials may prevent status reporting, even when a local build runs.

## Dashboard Access

Management and dashboard access require GitHub OAuth and an explicit operator allowlist. Use `githubOAuth.clientId`, `githubOAuth.existingSecret`, and `githubOAuth.allowedUsers` in Helm values. The existing Secret must contain `github-oauth-secret`. For loopback HTTP only, set `githubOAuth.secureCookies: false` and register the callback URL `http://localhost:8080/auth/github/callback` with the OAuth app.

No browser login is required for the signed local webhook above. `PUBLIC_READ` does not enable management operations.

## Remove the Local Release

```bash
make local-down
```

Review retained persistent volumes before deleting the minikube cluster.

## Local vs EKS Experiment

| | Local (minikube) | EKS example |
|---|---|---|
| **Temporal DB** | PostgreSQL subchart | RDS via ACK |
| **Secrets** | K8s Secrets | AWS Secrets Manager |
| **Logging** | stdout / kubectl logs | CloudWatch + S3 |
| **Compute** | Single node | EKS Auto Mode |
| **IAM** | N/A | EKS Pod Identity |

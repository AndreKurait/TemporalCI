---
title: Local Development
description: Run TemporalCI locally with minikube for development and testing.
---

## Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/)
- [Helm 3](https://helm.sh/docs/intro/install/)
- kubectl
- Docker
- Python 3

This is a POC stack with Temporal, PostgreSQL, LocalStack, and a CI worker. Use a disposable local cluster and trusted repositories. For code-only development, `make check` needs none of these services.

## Build and Start the Cluster

```bash
./scripts/local-up.sh
```

The script starts minikube if needed, enables its registry, builds and loads the worker/webhook image, fetches Helm dependencies, and installs into the `temporalci` namespace. Its default new cluster requests 8 CPUs and 16 GiB of memory. Start minikube yourself with suitable resources first if you need different settings.

Setup, credential preparation, and the trigger helper explicitly use the `minikube` Kubernetes context; they do not follow your current kubectl context.

On a fresh installation the script creates random credentials in the externally managed `temporalci-database` Secret (`postgres-password`) and `temporalci-local-secrets` Secret (`github-webhook-secret`, `github-token`). Reruns preserve existing values. The local values keep the services inside the cluster. Dependency or readiness failures stop the script; inspect `kubectl get pods -n temporalci` before retrying.

For older local releases, the script copies the password from `temporalci-postgresql` and copies the data from `temporalci-secrets` into the new external Secrets before Helm runs. This preserves the database password, registered webhook secret, token, and any OAuth key while allowing Helm to remove its former Secret resources. It does not rotate credentials or modify persistent volumes.

If an existing release or persistent volume has no recoverable credential Secret, setup stops. Restore the password actually used by the database into `temporalci-database` and the existing webhook credentials into `temporalci-local-secrets` through your secret-management process, then retry. A generated replacement password cannot unlock an initialized database. Back up retained data before a real upgrade; the migration helper has mocked tests, not a live upgrade compatibility guarantee.

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

The helper reads `temporalci-local-secrets` through kubectl and signs its request. Set `WEBHOOK_SECRET` explicitly when testing a different instance. It clones and executes the chosen repository; use one you control. This synthetic push has no changed-file list, so pipelines with a `paths` filter are skipped. Missing GitHub credentials may prevent status reporting, even when a local build runs.

## Dashboard Access

The local setup leaves `ciDashboard.enabled` disabled and builds only the worker/webhook image. To experiment with the SvelteKit dashboard, first build and load its image into minikube, then enable `ciDashboard.enabled`. The chart derives the dashboard image from the shared `image.registry`, `image.repository` (with a `-dashboard` suffix), and `image.tag` values. OAuth configuration alone does not deploy the dashboard.

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
| **Temporal DB** | PostgreSQL subchart | PostgreSQL subchart in the example; RDS/ACK requires separate configuration |
| **Secrets** | K8s Secrets | K8s Secrets in the example; AWS Secrets Manager integration is optional |
| **Logging** | Pod logs and LocalStack S3 | Pod logs; S3 needs a configured bucket and permissions |
| **Compute** | Local minikube cluster | Experimental EKS Auto Mode configuration |
| **IAM** | LocalStack development credentials | Roles and Pod Identity require environment-specific setup |

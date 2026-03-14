# TemporalCI

A Kubernetes-native CI system built on [Temporal](https://temporal.io/) for durable, scalable workflow orchestration. Runs on any K8s cluster — from local minikube to EKS Auto Mode in production.

[![CI](https://github.com/AndreKurait/TemporalCI/actions/workflows/ci.yml/badge.svg)](https://github.com/AndreKurait/TemporalCI/actions/workflows/ci.yml)
[![Docker Build](https://github.com/AndreKurait/TemporalCI/actions/workflows/docker-build.yml/badge.svg)](https://github.com/AndreKurait/TemporalCI/actions/workflows/docker-build.yml)

---

## Why TemporalCI?

Traditional CI systems (Jenkins, GitHub Actions runners) are stateless and fragile — a network blip kills your build, a timeout loses your progress, and debugging requires digging through opaque logs.

TemporalCI replaces that with **Temporal workflows** that are:

- **Durable** — workflows survive crashes and resume exactly where they left off
- **Retryable** — failed activities retry automatically with configurable policies
- **Observable** — every workflow execution is fully inspectable in the Temporal Web UI
- **Scalable** — CI jobs run as isolated K8s pods, scaling with your cluster

## How It Works

```
GitHub push/PR event
       │
       ▼
┌─────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Webhook    │────▶│  Temporal Server  │────▶│     Worker      │
│   Server     │     │                  │     │                 │
└─────────────┘     └──────────────────┘     └────────┬────────┘
                                                       │
                                              ┌────────▼────────┐
                                              │  CIPipeline      │
                                              │  Workflow         │
                                              │                  │
                                              │  1. CloneRepo    │
                                              │  2. RunStep (×N) │
                                              │  3. ReportResults│
                                              └────────┬────────┘
                                                       │
                                              ┌────────▼────────┐
                                              │  CI Job Pods     │
                                              │  (K8s)           │
                                              └────────┬────────┘
                                                       │
                                              ┌────────▼────────┐
                                              │  GitHub Check    │
                                              │  Run + PR Comment│
                                              └─────────────────┘
```

1. **Webhook server** receives GitHub `push` / `pull_request` events, validates signatures, and starts a Temporal workflow
2. **CIPipeline workflow** clones the repo, runs each step as a K8s pod, and reports results
3. **Results** appear as GitHub Check Runs with pass/fail annotations and a PR summary comment

---

## Quick Start

### Local Development (minikube)

```bash
# Start cluster
minikube start

# Install TemporalCI (includes Temporal server + PostgreSQL)
helm install temporalci ./deploy/helm -f deploy/helm/values-local.yaml

# Create secrets for GitHub integration
kubectl create secret generic temporalci-secrets \
  --from-literal=github-webhook-secret=dev-secret \
  --from-literal=github-token=ghp_your_token_here

# Access Temporal Web UI
kubectl port-forward svc/temporalci-temporal-web 8088:8088
# Open http://localhost:8088
```

### Production (EKS Auto Mode)

See [Production Deployment Guide](docs/production.md).

---

## Configuring Your Repo

Add a `.temporalci.yaml` to the root of any repository to define its CI pipeline:

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...

  - name: test
    image: golang:1.23
    command: go test ./... -v

  - name: lint
    image: golangci/golangci-lint:latest
    command: golangci-lint run
```

Each step runs in its own isolated K8s pod. If no `.temporalci.yaml` is found, TemporalCI uses a default Go build + test pipeline.

### Step Configuration

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Display name for the step |
| `image` | Yes | Docker image to run the step in |
| `command` | Yes | Shell command to execute |
| `timeout` | No | Step timeout (e.g., `5m`, `30m`) |

---

## Using the Reusable Workflow

TemporalCI provides a GitHub Actions reusable workflow for repos that want CI without a full Temporal deployment:

```yaml
# .github/workflows/ci.yml
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    uses: AndreKurait/TemporalCI/.github/workflows/reusable-ci.yml@main
    with:
      go-version: '1.23'          # optional, default: 1.23
      build-command: 'go build ./...'  # optional
      test-command: 'go test ./... -v -json'  # optional
```

This runs Build, Test, and Vet as parallel jobs with JUnit XML test reporting.

---

## Architecture

### Components

| Component | Description | Code |
|-----------|-------------|------|
| **Webhook Server** | HTTP server that receives GitHub events and starts Temporal workflows | [`cmd/webhook/`](cmd/webhook/) |
| **Worker** | Temporal worker that executes CI pipeline activities | [`cmd/worker/`](cmd/worker/) |
| **CIPipeline Workflow** | Orchestrates clone → build → test → report | [`internal/workflows/`](internal/workflows/) |
| **Activities** | CloneRepo, RunStep, ReportResults, UploadLog | [`internal/activities/`](internal/activities/) |
| **K8s Pod Runner** | Creates and manages CI job pods | [`internal/k8s/`](internal/k8s/) |
| **JUnit Parser** | Parses JUnit XML test results | [`internal/junit/`](internal/junit/) |
| **Pipeline Config** | Loads `.temporalci.yaml` from repos | [`internal/config/`](internal/config/) |

### Deployment Modes

| | Local (minikube) | Production (EKS Auto Mode) |
|---|---|---|
| **Install** | `helm install` | Argo CD (EKS Capability) |
| **Temporal DB** | PostgreSQL subchart | RDS via ACK |
| **Secrets** | K8s Secrets | Secrets Store CSI → AWS Secrets Manager |
| **CI Logs** | stdout | S3 + presigned URLs |
| **Compute** | Single node | Auto Mode with system + ci-jobs NodePools |
| **IAM** | N/A | EKS Pod Identity |

### CI Pipeline Flow

```
CIPipeline Workflow
  │
  ├── CloneRepo activity
  │     └── git clone --depth=1 into working directory
  │
  ├── RunStep activity (for each step in .temporalci.yaml)
  │     ├── K8s mode: creates a Pod on ci-jobs NodePool
  │     │     ├── Runs command in specified Docker image
  │     │     ├── Streams logs via K8s log API
  │     │     └── Collects exit code + JUnit XML
  │     └── Local mode: runs command via sh -c (no K8s required)
  │
  ├── UploadLog activity (production)
  │     └── Uploads full log to S3, returns presigned URL
  │
  └── ReportResults activity
        ├── Creates GitHub Check Run (pass/fail with summary)
        └── Posts PR comment with step-by-step results table
```

---

## Project Structure

```
TemporalCI/
├── cmd/
│   ├── worker/main.go           # Temporal worker entrypoint
│   └── webhook/main.go          # GitHub webhook HTTP server
├── internal/
│   ├── workflows/
│   │   ├── ci_pipeline.go       # CIPipeline workflow definition
│   │   └── types.go             # Input/output types
│   ├── activities/
│   │   ├── activities.go        # CloneRepo, RunStep, ReportResults
│   │   ├── s3.go                # UploadLog (S3 + presigned URLs)
│   │   └── types.go             # Activity input/output types
│   ├── k8s/
│   │   └── pod.go               # K8s pod create/watch/logs/cleanup
│   ├── junit/
│   │   └── parser.go            # JUnit XML parser
│   └── config/
│       ├── config.go            # App config from env vars
│       └── pipeline.go          # .temporalci.yaml loader
├── deploy/
│   ├── helm/                    # Umbrella Helm chart
│   │   ├── Chart.yaml           # Temporal + PostgreSQL subcharts
│   │   ├── values.yaml          # Default values
│   │   ├── values-local.yaml    # minikube overrides
│   │   ├── values-prod.yaml.example
│   │   └── templates/           # K8s manifests
│   └── terraform/               # EKS cluster bootstrap
│       ├── eks.tf               # EKS Auto Mode cluster
│       ├── iam.tf               # IAM roles + Pod Identity
│       ├── ecr.tf               # Container registry
│       └── variables.tf
├── docs/                        # Documentation
├── .github/workflows/           # CI/CD workflows
├── Dockerfile                   # Multi-stage Go build
├── Makefile
└── go.mod
```

---

## Configuration

### Environment Variables

The worker and webhook server are configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST_PORT` | `localhost:7233` | Temporal server address |
| `PORT` | `8080` | Webhook server listen port |
| `GITHUB_WEBHOOK_SECRET` | — | Secret for validating webhook signatures |
| `LOG_BUCKET` | — | S3 bucket for CI build logs |
| `AWS_REGION` | `us-east-1` | AWS region for S3/ECR operations |

The webhook server also reads secrets from file mounts at `/etc/temporalci/` (for Kubernetes Secrets / Secrets Store CSI compatibility).

### Helm Values

See [`deploy/helm/values.yaml`](deploy/helm/values.yaml) for all configurable values. Key sections:

- `image.*` — Container image settings
- `worker.*` — Worker replica count and resources
- `webhook.*` — Webhook server settings
- `temporal.*` — Temporal server subchart config
- `postgresql.*` — PostgreSQL subchart config
- `secrets.*` — Secret management (local K8s Secrets or AWS Secrets Manager)
- `rds.*` — RDS via ACK (production)
- `s3.*` — S3 bucket via ACK (production)
- `nodePool.*` — EKS NodePool configuration
- `serviceAccounts.*` — IAM role ARNs for Pod Identity

---

## Development

```bash
# Build all binaries
make build

# Run tests
make test

# Run linter
make lint

# Build Docker image locally
docker build -t temporalci .
```

### Running Locally Without K8s

The worker and webhook can run locally without Kubernetes. When `K8sClient` is nil, `RunStep` falls back to executing commands directly via `sh -c`. You just need a running Temporal server:

```bash
# Start Temporal dev server (install: https://docs.temporal.io/cli)
temporal server start-dev

# In another terminal, start the worker
TEMPORAL_HOST_PORT=localhost:7233 go run ./cmd/worker

# In another terminal, start the webhook server
TEMPORAL_HOST_PORT=localhost:7233 PORT=8080 go run ./cmd/webhook
```

---

## GitHub Secrets

See [docs/github-secrets.md](docs/github-secrets.md) for the full list of required GitHub Secrets.

## License

MIT

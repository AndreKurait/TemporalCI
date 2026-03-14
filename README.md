# TemporalCI

A Kubernetes-native CI system built on [Temporal](https://temporal.io/) for durable, scalable workflow orchestration.

## Overview

TemporalCI replaces fragile CI pipelines with Temporal workflows that are inherently retryable, observable, and durable. CI jobs run as Kubernetes pods orchestrated by Temporal workers.

## Quick Start (Local)

```bash
minikube start
helm install temporalci ./deploy/helm -f deploy/helm/values-local.yaml
```

## Architecture

- **Webhook Server** — receives GitHub events, starts Temporal workflows
- **Worker** — executes CI pipeline activities (clone, build, test, report)
- **Temporal** — durable workflow orchestration

## Development

```bash
make build    # Build all binaries
make test     # Run tests
make lint     # Run go vet
```

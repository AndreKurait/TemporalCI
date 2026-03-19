---
title: Helm Values Reference
description: Complete reference for TemporalCI Helm chart values.
---

## Core Values

```yaml
# Image configuration
image:
  registry: ""              # ECR registry URL (set at deploy time)
  repository: temporalci
  tag: latest               # Git SHA in production
  pullPolicy: IfNotPresent

# Webhook server
webhook:
  replicas: 1
  port: 8080
  resources:
    requests: { cpu: "100m", memory: "128Mi" }
    limits:   { cpu: "500m", memory: "256Mi" }

# Temporal worker
worker:
  replicas: 1
  resources:
    requests: { cpu: "250m", memory: "256Mi" }
    limits:   { cpu: "1",    memory: "512Mi" }
```

## AWS Configuration

```yaml
aws:
  region: ""                # Set via --set aws.region=us-east-1
  enabled: false            # Enable AWS integrations (Pod Identity, S3, etc.)

# Secrets management
secrets:
  aws:
    enabled: false          # Use Secrets Store CSI Driver
  prefix: temporalci        # AWS Secrets Manager prefix
```

## EKS Auto Mode

```yaml
autoMode:
  enabled: false            # Create NodePool CRDs

  nodePools:
    system:
      enabled: true
    ciJobs:
      enabled: true
      taint: "workload=ci-job:NoSchedule"
```

## ACK Resources (Production)

```yaml
ack:
  rds:
    enabled: false
    instanceClass: db.t4g.medium
    engine: postgres
    engineVersion: "16"
    allocatedStorage: 20
    encrypted: true

  s3:
    enabled: false
    # Bucket name set via --set ack.s3.bucketName=...
```

## Network Policies

```yaml
networkPolicy:
  enabled: false            # Enable CI pod network isolation
  ciPods:
    denyClusterInternal: true
    allowDNS: true
    allowHTTPS: true
```

## Temporal Subchart

TemporalCI includes the [official Temporal Helm chart](https://github.com/temporalio/helm-charts) as a dependency:

```yaml
temporal:
  server:
    replicaCount: 1
  web:
    enabled: true
    replicaCount: 1

# PostgreSQL subchart (local only)
postgresql:
  enabled: true             # Disable when using RDS
  auth:
    postgresPassword: temporal
    database: temporal
```

## Example: Local Development

```yaml
# values-local.yaml
image:
  tag: latest
  pullPolicy: Never         # Use local images

temporal:
  server:
    replicaCount: 1
  web:
    enabled: true

postgresql:
  enabled: true

secrets:
  aws:
    enabled: false

autoMode:
  enabled: false

networkPolicy:
  enabled: false
```

## Example: Production (EKS)

```bash
helm install temporalci ./deploy/helm \
  --namespace temporalci --create-namespace \
  --set image.registry="${ECR_REGISTRY}" \
  --set image.tag="${GIT_SHA}" \
  --set aws.enabled=true \
  --set aws.region=us-east-1 \
  --set secrets.aws.enabled=true \
  --set secrets.prefix=temporalci \
  --set autoMode.enabled=true \
  --set ack.rds.enabled=true \
  --set ack.s3.enabled=true \
  --set ack.s3.bucketName="${LOG_BUCKET}" \
  --set networkPolicy.enabled=true \
  --set postgresql.enabled=false
```

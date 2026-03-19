---
title: Pod Execution Model
description: How TemporalCI runs each CI step in an isolated Kubernetes pod.
---

## Overview

Every CI step in TemporalCI runs in its own Kubernetes pod. This provides:

- **Isolation** — steps can't interfere with each other
- **Resource limits** — each step gets defined CPU/memory
- **Any image** — use any Docker image for any step
- **Automatic cleanup** — pods are deleted after completion

## Pod Lifecycle

When the `RunStep` activity executes:

```
Worker creates Pod spec
        │
        ▼
K8s schedules pod on ci-jobs NodePool
        │
        ▼
Init container: git clone (shallow, depth=1)
        │
        ▼
Main container: run step command (sh -c)
        │
        ▼
Worker streams logs via K8s log API
        │
        ▼
Worker collects exit code
        │
        ▼
Worker deletes pod (cleanup)
```

## Node Pool Isolation

In production (EKS Auto Mode), CI pods run on a dedicated `ci-jobs` NodePool:

| NodePool | Taint | Workloads |
|----------|-------|-----------|
| `system` | (none) | Temporal server, workers, webhook |
| `ci-jobs` | `workload=ci-job:NoSchedule` | CI build/test pods only |

CI pods include the matching toleration and a `nodeSelector` to ensure they only land on CI nodes. This prevents CI workloads from competing with Temporal for resources.

## Pod Spec

A typical CI pod looks like:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ci-{workflow-id}-{step-name}
  labels:
    app: temporalci-job
spec:
  serviceAccountName: temporalci-ci-job  # zero K8s API permissions
  tolerations:
    - key: workload
      value: ci-job
      effect: NoSchedule
  nodeSelector:
    temporalci/pool: ci-jobs
  containers:
    - name: step
      image: golang:1.23          # from .temporalci.yaml
      command: ["sh", "-c"]
      args: ["go test ./..."]     # from .temporalci.yaml
      resources:
        requests: { cpu: "500m", memory: "512Mi" }
        limits:   { cpu: "2",    memory: "2Gi" }
      volumeMounts:
        - name: workspace
          mountPath: /workspace
  volumes:
    - name: workspace
      emptyDir: {}
  restartPolicy: Never
```

## Security

CI pods are locked down by default:

- **Zero K8s API access** — the `temporalci-ci-job` ServiceAccount has no RBAC permissions
- **Network isolation** — NetworkPolicy blocks all inbound traffic and egress to cluster-internal IPs (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16). Only DNS and HTTPS egress allowed
- **No host access** — no hostPath mounts, no privileged containers
- **Ephemeral** — pods are deleted immediately after the step completes

## Local vs Production

| | Local (minikube) | Production (EKS) |
|---|---|---|
| Node pool | Single node | Dedicated `ci-jobs` NodePool |
| Network policy | Not enforced (minikube) | Enforced by VPC CNI |
| Resources | Best-effort | Defined limits |
| Cleanup | Same (pod deletion) | Same (pod deletion) |

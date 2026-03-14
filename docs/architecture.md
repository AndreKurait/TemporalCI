# Architecture

## System Overview

```mermaid
graph TB
    subgraph K8s["Kubernetes Cluster"]
        WH["Webhook Server\nPOST /webhook\nGET /health"]
        TS["Temporal Server\nWorkflow History\nTask Queues"]
        PG["PostgreSQL\n(or RDS)"]
        WK["Worker\nCIPipeline\nCloneRepo\nRunStep\nReportResults\nUploadLog"]
        PODS["CI Job Pods\n(ephemeral)"]

        WH -->|start workflow| TS
        TS <-->|poll & complete| WK
        TS --> PG
        WK -->|create pods| PODS
    end

    GH["GitHub\nWebhooks\nCheck Runs\nPR Comments"]
    AWS["AWS\nS3 (logs)\nECR (images)\nSecrets Manager"]

    GH -->|events| WH
    WK -->|report| GH
    WK -->|upload logs| AWS
```

## Workflow Execution

The `CIPipeline` workflow is the core orchestration unit:

```mermaid
flowchart TD
    Start(["CIPipeline(repo, ref, sha, prNumber)"]) --> Clone

    Clone["1. CloneRepo\nInput: repo URL + git ref\nAction: git clone --depth=1, git checkout\nOutput: working directory path"]
    Clone --> Step

    Step["2. RunStep (for each step in .temporalci.yaml)\nInput: directory, command, image\nAction: Create K8s pod → run → stream logs → collect exit code\nOutput: exit code, stdout/stderr, JUnit XML\nRetry: up to 3 attempts, 10min timeout"]
    Step --> Upload

    Upload["3. UploadLog (production only)\nInput: log content, workflow ID\nAction: Upload to S3, generate presigned URL (1hr)\nOutput: presigned URL for Check Run details"]
    Upload --> Report

    Report["4. ReportResults\nInput: repo, SHA, PR number, step results\nAction: Create GitHub Check Run + PR comment\nOutput: check run ID"]
```

Each activity is independently retryable. If the worker crashes mid-pipeline, Temporal replays the workflow from the last completed activity.

## K8s Pod Lifecycle

When `RunStep` executes in K8s mode:

```mermaid
sequenceDiagram
    participant W as Worker
    participant K as K8s API
    participant P as CI Pod

    W->>K: Create Pod (image, command, ci-jobs toleration)
    K->>P: Schedule on ci-jobs NodePool
    P->>P: Execute CI command
    W->>K: Watch pod status
    K-->>W: Phase: Succeeded/Failed
    W->>K: Get pod logs (stream)
    K-->>W: stdout/stderr
    W->>K: Get terminated container status
    K-->>W: exit code
    W->>K: Delete pod (cleanup)
```

## Security Model

| Layer | Mechanism |
|-------|-----------|
| **Webhook validation** | HMAC-SHA256 signature verification on every GitHub event |
| **Secret storage** | File mounts from K8s Secrets (local) or AWS Secrets Manager (prod) |
| **IAM** | EKS Pod Identity — each component gets least-privilege IAM role |
| **Network isolation** | CI jobs run on dedicated `ci-jobs` NodePool with taints |
| **Container isolation** | Each CI step runs in its own ephemeral pod |

## Data Flow

```mermaid
flowchart LR
    A["GitHub Event"] --> B["Webhook Server"]
    B --> C["Temporal\n(enqueue)"]
    C --> D["Worker\n(dequeue)"]
    D --> E["Clone repo"]
    E --> F["Run steps\n(K8s pods)"]
    F --> G["Upload logs\n→ S3"]
    G --> H["Report\n→ GitHub API"]
```

No CI state is stored in the webhook server or worker — Temporal owns all execution state. Both components are stateless and horizontally scalable.

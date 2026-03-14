# Architecture

## System Overview

```
┌──────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                          │
│                                                               │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────────┐  │
│  │   Webhook    │    │   Temporal    │    │    Worker      │  │
│  │   Server     │───▶│   Server     │◀──▶│                │  │
│  │              │    │              │    │  Registers:     │  │
│  │  POST /webhook    │  Workflow    │    │  - CIPipeline   │  │
│  │  GET  /health│    │  History     │    │  - CloneRepo    │  │
│  └──────────────┘    │  Task Queues │    │  - RunStep      │  │
│                      └──────┬───────┘    │  - ReportResults│  │
│                             │            │  - UploadLog    │  │
│                      ┌──────▼───────┐    └───────┬────────┘  │
│                      │  PostgreSQL   │            │           │
│                      │  (or RDS)     │    ┌───────▼────────┐  │
│                      └──────────────┘    │  CI Job Pods    │  │
│                                          │  (ephemeral)    │  │
│                                          └────────────────┘  │
└──────────────────────────────────────────────────────────────┘
         │                                         │
         ▼                                         ▼
┌─────────────────┐                    ┌─────────────────────┐
│  GitHub          │                    │  AWS                 │
│  - Webhooks      │                    │  - S3 (logs)         │
│  - Check Runs    │                    │  - ECR (images)      │
│  - PR Comments   │                    │  - Secrets Manager   │
└─────────────────┘                    └─────────────────────┘
```

## Workflow Execution

The `CIPipeline` workflow is the core orchestration unit:

```
CIPipeline(repo, ref, sha, prNumber)
│
├─ 1. CloneRepo
│     Input:  repo URL + git ref
│     Action: git clone --depth=1, git checkout
│     Output: working directory path
│
├─ 2. RunStep (for each step in .temporalci.yaml)
│     Input:  directory, command, image
│     Action: Create K8s pod → run command → stream logs → collect exit code
│     Output: exit code, stdout/stderr, JUnit XML (if present)
│     Retry:  up to 3 attempts, 10-minute timeout per step
│
├─ 3. UploadLog (production only)
│     Input:  log content, workflow ID
│     Action: Upload to S3, generate presigned URL (1hr)
│     Output: presigned URL for Check Run details link
│
└─ 4. ReportResults
      Input:  repo, SHA, PR number, step results
      Action: Create GitHub Check Run + PR comment
      Output: check run ID
```

Each activity is independently retryable. If the worker crashes mid-pipeline, Temporal replays the workflow from the last completed activity.

## K8s Pod Lifecycle

When `RunStep` executes in K8s mode:

1. **Create** — Pod created in `temporalci` namespace with specified image and command
2. **Schedule** — Pod scheduled to `ci-jobs` NodePool (via toleration) for isolation
3. **Execute** — Container runs the CI command
4. **Stream** — Logs streamed via K8s pod log API
5. **Collect** — Exit code extracted from terminated container status
6. **Cleanup** — Pod deleted (always, even on failure)

## Security Model

| Layer | Mechanism |
|-------|-----------|
| **Webhook validation** | HMAC-SHA256 signature verification on every GitHub event |
| **Secret storage** | File mounts from K8s Secrets (local) or AWS Secrets Manager (prod) |
| **IAM** | EKS Pod Identity — each component gets least-privilege IAM role |
| **Network isolation** | CI jobs run on dedicated `ci-jobs` NodePool with taints |
| **Container isolation** | Each CI step runs in its own ephemeral pod |

## Data Flow

```
GitHub Event → Webhook Server → Temporal (enqueue) → Worker (dequeue)
                                                         │
                                                    Clone repo
                                                         │
                                                    Run steps (K8s pods)
                                                         │
                                                    Upload logs → S3
                                                         │
                                                    Report → GitHub API
```

No CI state is stored in the webhook server or worker — Temporal owns all execution state. Both components are stateless and horizontally scalable.

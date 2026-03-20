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

## CI Dashboard

The CI Dashboard is a SvelteKit application (`ui/` directory) that provides a web UI for monitoring builds, repos, and analytics.

```mermaid
graph LR
    Browser["Browser"] --> Nginx["Nginx Proxy\n(port 80)"]
    Nginx -->|"/ci/*"| Dashboard["CI Dashboard\n(SvelteKit, port 3000)"]
    Nginx -->|"/api/ci/*\n/auth/*"| WH["Webhook Server\n(port 8080)"]
    Nginx -->|"/*"| WH
    Dashboard -->|"SSR fetch"| WH
```

### Components

| Component | Role |
|-----------|------|
| `ui/` (SvelteKit) | Server-rendered dashboard. Pages: builds, build detail, repos, triggers, analytics |
| Webhook server `/api/ci/*` | CI API layer — queries Temporal workflow history for build data |
| Webhook server `/auth/*` | GitHub OAuth login, session management |
| Nginx ConfigMap | Routes `/ci/*` to dashboard, `/api/ci/*` and `/auth/*` to webhook server |

### Authentication

GitHub OAuth with session cookies. The webhook server handles the OAuth flow:

1. `GET /auth/github` → redirects to GitHub authorize URL
2. `GET /auth/github/callback` → exchanges code for token, creates session
3. `GET /auth/me` → returns current user info
4. `POST /auth/logout` → clears session

When `PUBLIC_READ=true`, read-only API endpoints (`/api/ci/builds`, `/api/ci/repos`, `/api/ci/analytics`) are accessible without authentication. Write endpoints (e.g., marking notifications read) always require auth.

### Nginx Proxy Routing

The helm chart deploys an nginx ConfigMap that routes traffic:

| Path | Backend | Notes |
|------|---------|-------|
| `/ci/*` | CI Dashboard (port 3000) | WebSocket upgrade for HMR in dev |
| `/api/ci/*` | Webhook server (port 8080) | CI API endpoints |
| `/auth/*` | Webhook server (port 8080) | OAuth flow |
| `/_app/immutable/*` | Webhook server | Cached 7 days with `immutable` header |
| `/api/*` | Webhook server | Other API endpoints (repos, triggers, locks) |
| `/*` | Webhook server | Catch-all |

### Notification System

Build failure and recovery notifications flow through two channels:

1. **Slack** — The `NotifySlack` activity sends messages to a per-repo webhook URL (configured in repo registration). Includes repo, ref, status, step count, duration, and a link to the Temporal workflow.

2. **In-app** — The `NotificationStore` (in-memory, capped at 100 entries) tracks `build_failed` and `build_recovered` events. The dashboard polls `GET /api/ci/notifications` and users can mark notifications read via `POST /api/ci/notifications/read`.

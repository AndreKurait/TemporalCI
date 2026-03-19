---
title: Workflow Execution
description: How the CIPipeline Temporal workflow orchestrates CI steps.
---

## CIPipeline Workflow

The `CIPipeline` workflow is the core orchestration unit. It receives a webhook event and runs the full CI pipeline:

1. **CloneRepo** — `git clone --depth=1` + checkout the target ref
2. **RunStep** (for each step) — create a K8s pod, run the command, stream logs, collect exit code
3. **UploadLog** (production) — upload full logs to S3, generate presigned URL
4. **ReportResults** — create GitHub commit status + PR comment with results

Each activity is independently retryable. If the worker crashes mid-pipeline, Temporal replays the workflow from the last completed activity.

## K8s Pod Lifecycle

When `RunStep` executes in K8s mode:

1. Worker creates a Pod via the K8s API (with image, command, ci-jobs toleration)
2. K8s schedules the pod on the `ci-jobs` NodePool
3. Pod executes the CI command
4. Worker watches pod status and streams logs
5. Worker collects the exit code from the terminated container
6. Worker deletes the pod (cleanup)

## Cancellation

When a new push arrives on the same branch, TemporalCI:

1. Cancels the previous workflow via Temporal's cancellation mechanism
2. The cancelled workflow runs a **disconnected context** reporting activity
3. This reports the cancellation to GitHub (so the PR shows "cancelled", not "pending forever")
4. Cleanup activities remove clone directories and stale pods

## Retry Behavior

| Activity | Retries | Timeout | Notes |
|----------|---------|---------|-------|
| CloneRepo | 3 | 5m | Handles transient git/network failures |
| RunStep | 1 | Per-step config | Respects `.temporalci.yaml` timeout |
| UploadLog | 2 | 2m | S3 upload with presigned URL |
| ReportResults | 3 | 1m | GitHub API can be flaky |

## Workflow Query

Active workflows expose a query handler that returns real-time status:

```go
// Query "status" returns current pipeline state
workflow.SetQueryHandler(ctx, "status", func() (PipelineStatus, error) {
    return currentStatus, nil
})
```

This powers the Temporal Web UI status display and can be used for external integrations.

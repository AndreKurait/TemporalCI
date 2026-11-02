---
title: Why Temporal for CI?
description: What Temporal contributes to CI orchestration and what this POC still needs.
---

## The Core Problem

CI orchestration needs to track work across asynchronous operations and infrastructure failures. TemporalCI explores using Temporal's persisted workflow history for those decisions. Recovering the actual build process and its filesystem state is a separate problem.

## What Temporal Gives Us

[Temporal](https://temporal.io/) is a durable execution platform. It was built to solve exactly this class of problem for microservices. TemporalCI applies the same primitives to CI/CD:

### Workflow Replay

Temporal can replay recorded workflow history on a new worker, using previously recorded activity results. This preserves orchestration decisions, not worker-local files or running shell commands.

TemporalCI currently passes a worker-local checkout path between activities. A replacement worker may not have that directory. Ordinary `RunStep` activities also have one attempt, so an interrupted step is not automatically retried. These are explicit POC limitations.

### Per-Activity Retries

Each CI step is a Temporal activity with its own retry policy:

| Activity | Retries | Backoff | Why |
|----------|---------|---------|-----|
| CloneRepo | 3 | 5s → 30s | Git/network failures are transient |
| RunStep | 1 | — | Build failures are usually real |
| ReportResults | 3 | 1s → 5s | GitHub API can be flaky |
| UploadLog | 2 | 2s → 10s | S3 uploads occasionally fail |

Retries depend on each activity's configured policy. Repeated side effects must be idempotent; Temporal history alone does not make an external build command safe to repeat.

### Cancellation with Cleanup

When a new push arrives on the same branch, TemporalCI cancels the previous workflow. But cancellation in Temporal is graceful — the workflow gets a cancellation signal and can run cleanup logic in a **disconnected context**:

1. Cancel the running workflow
2. Workflow catches the cancellation
3. Cleanup activity runs (delete pods, report "cancelled" to GitHub)
4. Attempt to report the final status to GitHub

Cleanup and status reporting are best effort. They can fail if Kubernetes, AWS, or GitHub is unavailable.

### Full Observability

Every workflow execution is recorded in Temporal's history. You can:

- See the exact input and output of every activity
- See timing for each step
- Replay any workflow execution
- Query running workflows for real-time status

The Temporal Web UI gives you this for free — no custom dashboards needed.

## Why Not Just Use Temporal Directly?

You could write Temporal workflows for CI yourself. TemporalCI is the opinionated layer on top:

- **`.temporalci/default.yaml`** — declarative pipeline config instead of writing Go code
- **GitHub integration** — webhook handling, commit status, PR comments
- **K8s pod execution** — each step in an isolated pod with resource limits
- **Helm chart** — one install gets Temporal + workers + webhook server
- **Security controls** — operator authentication and optional cluster policies, with limitations described in the security model

The goal is a small, inspectable experiment in workflow-based CI orchestration. It is not a production support or reliability guarantee.

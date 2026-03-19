---
title: Why Temporal for CI?
description: How Temporal's durable execution model solves CI's fundamental problems.
---

## The Core Problem

Every CI system treats pipelines as **ephemeral processes**. A pipeline starts, runs steps, and either succeeds or fails. If anything goes wrong mid-execution — node crash, network timeout, OOM kill — the entire pipeline is lost. You start over.

This is the same problem that plagued microservice orchestration before Temporal: long-running processes that need to survive infrastructure failures.

## What Temporal Gives Us

[Temporal](https://temporal.io/) is a durable execution platform. It was built to solve exactly this class of problem for microservices. TemporalCI applies the same primitives to CI/CD:

### Workflow Replay

When a Temporal worker crashes, the workflow doesn't die. Temporal replays the workflow history on a new worker, skipping already-completed activities, and resumes from exactly where it left off.

For CI, this means: if your worker node gets evicted mid-build, the pipeline resumes from the last completed step. The clone doesn't re-run. The tests that already passed don't re-run. Only the interrupted step retries.

### Per-Activity Retries

Each CI step is a Temporal activity with its own retry policy:

| Activity | Retries | Backoff | Why |
|----------|---------|---------|-----|
| CloneRepo | 3 | 5s → 30s | Git/network failures are transient |
| RunStep | 1 | — | Build failures are usually real |
| ReportResults | 3 | 1s → 5s | GitHub API can be flaky |
| UploadLog | 2 | 2s → 10s | S3 uploads occasionally fail |

This is fundamentally different from "re-run the whole pipeline." Only the failed activity retries, with configurable backoff.

### Cancellation with Cleanup

When a new push arrives on the same branch, TemporalCI cancels the previous workflow. But cancellation in Temporal is graceful — the workflow gets a cancellation signal and can run cleanup logic in a **disconnected context**:

1. Cancel the running workflow
2. Workflow catches the cancellation
3. Cleanup activity runs (delete pods, report "cancelled" to GitHub)
4. GitHub PR shows "cancelled" — not "pending forever"

### Full Observability

Every workflow execution is recorded in Temporal's history. You can:

- See the exact input and output of every activity
- See timing for each step
- Replay any workflow execution
- Query running workflows for real-time status

The Temporal Web UI gives you this for free — no custom dashboards needed.

## Why Not Just Use Temporal Directly?

You could write Temporal workflows for CI yourself. TemporalCI is the opinionated layer on top:

- **`.temporalci.yaml`** — declarative pipeline config instead of writing Go code
- **GitHub integration** — webhook handling, commit status, PR comments
- **K8s pod execution** — each step in an isolated pod with resource limits
- **Helm chart** — one install gets Temporal + workers + webhook server
- **Security defaults** — pod isolation, network policies, RBAC

TemporalCI is to Temporal what GitHub Actions is to Azure Pipelines — a focused, opinionated CI experience built on a powerful execution engine.

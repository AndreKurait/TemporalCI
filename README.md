# TemporalCI

**The CI system that never loses your build.**

TemporalCI is a Kubernetes-native CI/CD platform built on [Temporal](https://temporal.io/) that treats every pipeline as a durable, observable, replayable workflow. Push code, get results — even if the infrastructure underneath crashes mid-build.

[![Docker Build & Deploy](https://github.com/AndreKurait/TemporalCI/actions/workflows/docker-build.yml/badge.svg)](https://github.com/AndreKurait/TemporalCI/actions/workflows/docker-build.yml)
[![Terraform Validate](https://github.com/AndreKurait/TemporalCI/actions/workflows/terraform-validate.yml/badge.svg)](https://github.com/AndreKurait/TemporalCI/actions/workflows/terraform-validate.yml)

---

## The Problem

Every CI system today has the same fundamental flaw: **pipelines are ephemeral processes.** A network blip kills your 45-minute build. A node eviction loses your test results. A timeout means starting over from scratch. And when something fails, you're left digging through opaque logs with no way to replay or inspect what happened.

Jenkins solved this with persistent state — but at the cost of a monolithic, plugin-hell architecture that's impossible to scale. GitHub Actions and GitLab CI went stateless — fast to set up, but fragile at scale. Neither can answer the question: *"What was the exact state of my pipeline 3 steps in when it failed?"*

## The Insight

**Temporal already solved this problem for microservices.** Durable execution, automatic retries, workflow replay, full observability — these are table stakes in the Temporal ecosystem. TemporalCI applies the same primitives to CI/CD:

| Traditional CI | TemporalCI |
|---|---|
| Pipeline dies on crash | Workflow resumes exactly where it left off |
| Retry = start over | Retry = re-execute only the failed activity |
| Logs disappear | Every execution is fully inspectable and replayable |
| Opaque failure modes | Structured error handling with compensation logic |
| Scaling = more runners | Scaling = more K8s pods, auto-provisioned |
| Static infrastructure | Ephemeral EKS clusters, provisioned on demand |

## What Makes This Different

### 1. Durable Pipelines
Your CI pipeline is a Temporal workflow. If the worker crashes mid-build, Temporal replays the workflow history and resumes from the last completed activity. No lost work. No re-running passed steps.

### 2. Ephemeral Cluster Pool
Need to test a Helm chart on a real Kubernetes cluster? TemporalCI maintains a **warm pool of EKS clusters** that are leased to pipelines on demand. Your chart gets installed, tested, and validated on an isolated cluster — then the cluster is released back to the pool. No shared state. No test pollution.

```yaml
steps:
  - name: helm-test
    helm_test:
      chart_path: deploy/helm
      release_name: my-app
      namespace: test
      timeout: 10m
```

### 3. DAG-Based Step Execution
Steps declare dependencies. Independent steps run in parallel. Failed dependencies skip downstream steps. Each step runs in its own isolated K8s pod with its own image.

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...

  - name: unit-test
    image: golang:1.23
    command: go test ./...
    depends_on: [build]

  - name: lint
    image: golangci/golangci-lint:latest
    command: golangci-lint run
    depends_on: [build]

  - name: integration-test
    image: golang:1.23
    command: go test -tags=integration ./...
    depends_on: [unit-test]
```

### 4. GitHub-Native Reporting
Results appear directly on your PR — commit status, collapsible step logs, timing, and a link to the Temporal Web UI for deep inspection.

```
## TemporalCI Results

✅ build (12.5s)
✅ test (20.2s)
✅ vet (16.1s)

3 passed, 0 failed in 48.8s

🔗 View workflow run
```

### 5. Zero Stored Credentials
GitHub Actions authenticates to AWS via OIDC federation. No access keys. No rotation. No secrets to leak. The IAM trust policy is scoped to a single GitHub repo.

---

## Architecture

```
GitHub webhook → Webhook Server → Temporal Server → Worker → K8s Pods
                                                          → Cluster Pool (EKS)
                                                          → S3 (logs)
                                                          → GitHub API (results)
```

| Component | What It Does |
|-----------|-------------|
| **Webhook Server** | Validates GitHub signatures, starts Temporal workflows, serves admin dashboard |
| **Worker** | Executes CI activities — clone, run steps as K8s pods, report results |
| **CIPipeline** | Workflow: clone → run steps (DAG) → upload logs → report → cleanup |
| **ClusterPool** | Long-running workflow: maintains warm EKS clusters, handles lease/release signals |
| **HelmTestPipeline** | Workflow: lease cluster → clone → helm install → helm test → report → release |
| **PodCleanup** | Scheduled workflow: garbage-collects stale CI pods every hour |

### Key Design Decisions

- **Temporal for orchestration** — not a custom state machine. Workflows are plain Go functions with full replay semantics.
- **K8s pods for isolation** — each CI step runs in its own pod with its own image. No shared filesystem, no container reuse.
- **EKS Auto Mode for compute** — nodes are provisioned on demand. No idle capacity. No node management.
- **ArgoCD for deployment** — GitOps. Push to main → ECR → ArgoCD syncs → zero-downtime rollout.
- **OIDC for auth** — GitHub Actions → AWS IAM. No stored credentials anywhere.

---

## Quick Start

### 1. Configure Your Repo

Add `.temporalci.yaml` to any repository:

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...
    timeout: 5m

  - name: test
    image: golang:1.23
    command: go test -v ./...
    timeout: 5m
    depends_on: [build]
```

### 2. Register the Webhook

```bash
# Via the API
curl -X POST http://<webhook-url>/api/repos \
  -H "Content-Type: application/json" \
  -d '{"fullName": "owner/repo", "defaultBranch": "main"}'
```

### 3. Push Code

Every push and PR triggers a pipeline. Results appear on the commit and PR within seconds of completion.

---

## Production Deployment

TemporalCI runs on EKS with the following infrastructure (all managed via Terraform):

- **EKS cluster** with Auto Mode (system + CI node pools)
- **ECR** for container images
- **S3** for CI build logs with presigned URLs
- **RDS** for Temporal persistence (via ACK)
- **IAM roles** with Pod Identity (least privilege per component)
- **OIDC federation** for GitHub Actions (zero stored credentials)

```bash
cd deploy/terraform
terraform init
terraform apply
```

See [docs/github-oidc-bootstrap.md](docs/github-oidc-bootstrap.md) for the one-time OIDC setup.

---

## Project Structure

```
cmd/
  worker/          Temporal worker entrypoint
  webhook/         GitHub webhook HTTP server
internal/
  workflows/       CIPipeline, ClusterPool, HelmTestPipeline, PodCleanup, ApprovalGate
  activities/      CloneRepo, RunStep, ReportResults, ProvisionCluster, RunHelmTest, ...
  k8s/             Pod creation, log streaming, cleanup
  config/          App config + .temporalci.yaml parser
  ghapp/           GitHub App authentication
  metrics/         Prometheus metrics (ci_pods_active, ci_step_status_total)
deploy/
  helm/            Umbrella Helm chart (Temporal + PostgreSQL subcharts)
  terraform/       EKS, ECR, IAM, VPC, OIDC provider
docs/              Architecture, production guide, OIDC bootstrap
```

---

## Why Temporal?

Temporal is an open-source durable execution platform. It guarantees that workflows run to completion, even across process restarts, infrastructure failures, and network partitions. For CI/CD, this means:

- A 2-hour integration test suite **survives worker restarts** without re-running passed tests
- A flaky `git clone` **retries automatically** with exponential backoff
- A cancelled pipeline **runs cleanup logic** (delete pods, release clusters) even after cancellation
- Every pipeline execution is **fully replayable** — you can inspect the exact sequence of activities, their inputs, outputs, and timing in the Temporal Web UI

This isn't a wrapper around Temporal. The CI pipeline *is* a Temporal workflow. The cluster pool *is* a Temporal workflow. The approval gate *is* a Temporal signal. The entire system is ~3,000 lines of Go that leverages Temporal's guarantees instead of reimplementing them.

---

## Status

**Live and running** on EKS in `us-east-1`. The [TemporalCI-test](https://github.com/AndreKurait/TemporalCI-test) repo has an active webhook — every push and PR triggers a real CI pipeline that clones, builds, tests, and reports results back to GitHub.

## License

MIT

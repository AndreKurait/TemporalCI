# Pipeline Configuration

## Directory Format

TemporalCI reads pipeline definitions from the `.temporalci/` directory. Each `.yaml` file becomes a named pipeline (filename without extension):

```
.temporalci/
  ci.yaml              → pipeline "ci"
  terraform-apply.yaml → pipeline "terraform-apply"
  deploy-site.yaml     → pipeline "deploy-site"
```

Each file defines a single pipeline with triggers, steps, parameters, and post hooks.

## Triggers

The `on` block controls when a pipeline runs. Omit `on` to run on every event.

```yaml
on:
  push:
    branches: [main, develop]
    tags: ["v*"]
    paths: ["src/**", "*.go"]
  pull_request:
    branches: [main]
    labels: [ci-run]
    paths: ["src/**"]
  schedule:
    - cron: "0 2 * * *"
  release:
    types: [published]
  webhook:
    match:
      action: deploy
  issues:
    types: [opened, labeled]
```

| Trigger | Description |
|---------|-------------|
| `push` | Branch pushes and tag pushes. Filter by `branches`, `tags`, `paths` |
| `pull_request` | PR opened, synchronized, labeled, closed. Filter by `branches`, `labels`, `paths` |
| `schedule` | Cron-based. Uses Temporal schedules |
| `release` | GitHub release events. Filter by `types` (published, created, etc.) |
| `webhook` | Custom webhook via `POST /webhook/custom/{owner}/{repo}`. Match payload fields |
| `issues` | Issue lifecycle events. Filter by `types` |

### Path Filtering

Push and PR triggers support `paths` to skip pipelines when no relevant files changed. Patterns: exact paths, `src/**` (recursive), `*.go` (extension), `docs/*` (single level).

```yaml
on:
  push:
    branches: [main]
    paths: ["src/**", "deploy/**"]
```

## Steps

```yaml
steps:
  - name: test
    image: golang:1.24
    command: go test ./...
    timeout: 10m
```

### Step Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Step name (required, shown in GitHub Check Run) |
| `image` | string | Docker image |
| `command` | string | Shell command (`sh -c`) |
| `commands` | list | Multiple commands (joined with `&&`) |
| `timeout` | string | Max execution time (default: `10m`) |
| `depends_on` | list | Step names this step waits for |
| `resources` | object | `cpu` and `memory` limits |
| `secrets` | list | Secret names to inject as env vars |
| `when` / `if` | string | Conditional expression |
| `type` | string | `"gate"` for gate steps |
| `docker` | bool | Enable Docker-in-Docker |
| `privileged` | bool | Run in privileged mode |
| `services` | list | Sidecar service containers |
| `matrix` | object | Per-step matrix build |
| `dynamic_matrix` | string | Command that outputs matrix JSON |
| `artifacts` | object | Upload/download artifacts |
| `lock` | string | Named resource lock |
| `lock_pool` | object | Pool-based lock |
| `lock_timeout` | string | Max wait for lock acquisition |
| `aws_role` | object | IAM role assumption |
| `trigger` | object | Trigger a child pipeline |
| `allow-skip` | bool | Allow skipping via path filters |
| `post` | list | Post-step hooks |

## Conditional Execution

Use `when` (or `if`) to conditionally run steps. Expressions reference environment variables and pipeline context:

```yaml
- name: deploy
  when: "event == 'push' && ref == 'refs/heads/main'"
  image: alpine
  command: ./deploy.sh

- name: pr-comment
  if: "event == 'pull_request'"
  image: alpine
  command: echo "PR build"
```

Supported operators: `==`, `!=`, `&&`, `||`, `!`, `contains`, `startsWith`.

Available variables: `event`, `ref`, plus all parameters and `TEMPORALCI_*` env vars.

## Gate Steps

Gate steps act as synchronization points — they produce no work but depend on other steps:

```yaml
- name: all-checks
  type: gate
  depends_on: [test, lint, build]

- name: deploy
  depends_on: [all-checks]
  command: ./deploy.sh
```

## Matrix Builds

### Static Matrix

Define dimensions at the pipeline or step level. Each combination runs as a parallel child workflow:

```yaml
matrix:
  go: ["1.23", "1.24"]
  os: ["ubuntu", "alpine"]
  exclude:
    - go: "1.23"
      os: alpine
  fail_fast: true
  max_parallel: 4
```

Matrix variables are available as env vars (`$go`, `$os`).

### Dynamic Matrix

Generate matrix dimensions at runtime from a command's JSON output:

```yaml
- name: test
  dynamic_matrix: "cat matrix.json"
  image: golang:$go
  command: go test ./...
```

The command must output JSON matching the matrix dimensions format.

### Per-Step Matrix

```yaml
steps:
  - name: test
    matrix:
      version: ["3.10", "3.11", "3.12"]
    image: python:$version
    command: pytest
```

## Service Containers

Run sidecar containers alongside a step (e.g., databases for integration tests):

```yaml
- name: integration-test
  image: golang:1.24
  command: go test -tags=integration ./...
  services:
    - name: postgres
      image: postgres:16
      ports: [5432]
      env:
        POSTGRES_PASSWORD: test
      health:
        cmd: "pg_isready -U postgres"
        interval: 2s
        retries: 10
    - name: redis
      image: redis:7
      ports: [6379]
```

## Docker-in-Docker

Enable Docker socket access for steps that build/push images:

```yaml
- name: docker-build
  docker: true
  image: docker:27
  command: docker build -t myapp .
```

## Privileged Mode

Run a step container in privileged mode (required for some build tools):

```yaml
- name: build
  privileged: true
  image: moby/buildkit
  command: buildctl build ...
```

## Artifacts

### Upload

```yaml
- name: build
  image: golang:1.24
  command: go build -o /artifacts/myapp ./cmd/myapp
  artifacts:
    upload:
      - path: /artifacts/myapp
      - path: coverage.out
```

### Download

Download artifacts from a previous step or a different pipeline:

```yaml
- name: deploy
  depends_on: [build]
  artifacts:
    download:
      - from_step: build
        path: /artifacts/myapp
      - from_pipeline: build-pipeline
        path: /artifacts/config.json
```

Artifacts are stored in S3 with 24-hour presigned URLs.

## Parameters

Define pipeline parameters for manual triggers and webhooks:

```yaml
parameters:
  - name: ACTION
    type: choice
    default: plan
    options: [plan, apply, destroy]
    description: "Terraform action"
  - name: DRY_RUN
    type: boolean
    default: "true"
  - name: VERSION
    type: string
    default: "latest"
```

| Type | Description |
|------|-------------|
| `string` | Free-form text |
| `choice` | Must be one of `options` |
| `boolean` | `"true"` or `"false"` |

Parameters are injected as environment variables. Trigger via API:

```bash
curl -X POST /api/trigger/owner/repo/pipeline \
  -d '{"ref": "main", "parameters": {"ACTION": "apply"}}'
```

## Post Hooks

Run steps after the pipeline completes, regardless of success/failure:

```yaml
post:
  always:
    - name: cleanup
      image: alpine
      command: rm -rf /tmp/build
  on_failure:
    - name: notify
      image: alpine
      command: curl -X POST $SLACK_WEBHOOK -d '{"text":"Build failed"}'
      secrets: [SLACK_WEBHOOK]
```

## AWS Role Assumption

Assume an IAM role for a step. Supports credential chaining:

```yaml
- name: deploy
  aws_role:
    arn: arn:aws:iam::123456789012:role/deploy-role
    duration: 3600
    session_name: ci-deploy
  command: aws ecs update-service ...

- name: cross-account
  aws_role:
    arn: arn:aws:iam::987654321098:role/target-role
    source_credentials: deploy  # chain from the "deploy" step's credentials
  command: aws s3 sync ...
```

Credentials are injected as `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`.

## Lock Pools

Prevent concurrent access to shared resources. A long-running Temporal workflow manages lock state.

### Named Lock

```yaml
- name: deploy-prod
  lock: production
  lock_timeout: 10m
  command: ./deploy.sh
```

### Pool Lock

Acquire one resource from a pool (e.g., one of N test environments):

```yaml
- name: integration-test
  lock_pool:
    label: test-envs
    quantity: 1
  lock_timeout: 15m
  command: ./test.sh
```

Register pools via API:

```bash
curl -X POST /api/lock-pools -d '{"label":"test-envs","resources":["env-1","env-2","env-3"]}'
```

## Child Pipeline Triggers

Trigger another pipeline from a step:

```yaml
- name: deploy
  trigger:
    pipeline: deploy-prod
    parameters:
      VERSION: "$TEMPORALCI_SHA"
    wait: true
    propagate_failure: true
```

| Field | Description |
|-------|-------------|
| `pipeline` | Name of the pipeline to trigger |
| `parameters` | Parameters to pass |
| `wait` | Block until child completes (default: false) |
| `propagate_failure` | Fail parent if child fails (default: false) |

## JUnit XML

Output JUnit XML to `/tmp/test-results/*.xml` for rich test reporting in GitHub Check Runs:

```yaml
- name: test
  image: golang:1.24
  command: gotestsum --junitfile /tmp/test-results/results.xml -- ./... -v
```

## CI Dashboard

TemporalCI includes a web dashboard for monitoring builds. See [ci-dashboard.md](ci-dashboard.md) for details.

Pages: builds list, build detail with step logs, repos overview, manual triggers, analytics.

## Default Pipeline

If no `.temporalci/` directory is found, TemporalCI uses a default pipeline:

```yaml
steps:
  - name: build
    image: golang:1.23
    command: go build ./...
  - name: test
    image: golang:1.23
    command: go test ./...
```

## Full Example

```yaml
# .temporalci/ci.yaml
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

steps:
  - name: test
    image: golang:1.24
    command: go test -race ./...
    timeout: 10m
    artifacts:
      upload:
        - path: coverage.out

  - name: build
    image: golang:1.24
    depends_on: [test]
    commands:
      - CGO_ENABLED=0 go build -o /artifacts/webhook ./cmd/webhook
      - CGO_ENABLED=0 go build -o /artifacts/worker ./cmd/worker

  - name: docker-push
    depends_on: [build]
    when: "event == 'push'"
    docker: true
    image: docker:27
    command: docker build -t myapp . && docker push myapp
    secrets: [ECR_REGISTRY]
    aws_role:
      arn: arn:aws:iam::123456789012:role/ecr-push

  - name: all-checks
    type: gate
    depends_on: [test, build]

post:
  on_failure:
    - name: notify
      command: curl -X POST $SLACK_WEBHOOK -d '{"text":"CI failed"}'
      secrets: [SLACK_WEBHOOK]
```

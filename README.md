# TemporalCI

**A proof of concept for CI orchestration with Temporal and Kubernetes.**

TemporalCI explores using Temporal workflows to coordinate repository checkout, build steps, GitHub reporting, and cleanup. Pipelines are configured in `.temporalci/*.yaml`; a Go webhook server starts workflows, a Go worker executes activities, and a SvelteKit dashboard displays build history.

This is an experimental project, not a production-ready hosted CI service. Use a disposable environment and repositories you trust. See the [security model and limitations](SECURITY.md) before connecting credentials or accepting pull-request builds.

## What is implemented

- GitHub webhook handling, commit status and Check Run reporting.
- Pipeline configuration with dependencies, conditions, matrix child workflows, parameters, and post steps.
- Kubernetes build pods, optional service containers, artifacts, and S3 log uploads.
- Experimental EKS cluster-pool and Helm-test workflows.
- A dashboard for builds, repositories, logs, triggers, and notifications.

Temporal persists workflow decisions and activity results. It does **not** preserve worker-local checkout directories or guarantee that a running shell command survives a worker or pod failure. Ordinary `RunStep` activities currently have one attempt; normal steps execute in declaration order, while matrix and named-pipeline child workflows can run concurrently. Cleanup is best effort and can fail if its dependencies are unavailable.

## Start with local checks

The unit tests and workflow test environment do not require AWS credentials, a Kubernetes cluster, or a running Temporal server.

Prerequisites: Git, Go 1.24.1 or newer, and Node.js 24 with npm for the dashboard and documentation site.

```bash
git clone https://github.com/AndreKurait/TemporalCI.git
cd TemporalCI
make check

# Optional: build the dashboard and documentation site
make dashboard-install dashboard-build
npm --prefix site ci
npm --prefix site run build
```

`make check` builds the Go packages, runs `go vet`, and runs the race-enabled test suite. The [CI workflow](.github/workflows/ci.yml) runs these checks on pull requests and builds both web projects without cloud credentials or deployment permissions.

To validate pipeline configuration without executing it:

```bash
go run ./cmd/validate .
```

## Example pipeline

Save this as `.temporalci/default.yaml` in your repository. Each YAML filename defines a named pipeline.

```yaml
steps:
  - name: build
    image: golang:1.26
    command: go build ./...
    timeout: 5m

  - name: test
    image: golang:1.26
    command: go test ./...
    depends_on: [build]
    timeout: 10m
```

See the [pipeline reference](docs/pipeline-config.md) for additional options. Dependencies must appear before their dependents in the current executor.

## Run the POC locally

For an isolated Kubernetes experiment, install Docker, minikube, Helm, and kubectl, then follow the [local development guide](site/src/content/docs/getting-started/local-dev.md). The local stack also runs Temporal, PostgreSQL, and LocalStack; allow sufficient memory.

The native webhook process binds to `127.0.0.1` by default. Helm explicitly binds it inside the pod and uses a `ClusterIP` service. GitHub webhooks require `GITHUB_WEBHOOK_SECRET`; an unset secret rejects requests. Dashboard and management access require GitHub OAuth and a `GITHUB_ALLOWED_USERS` operator allowlist. Build-read access is private by default; public badges expose build status and whether a repository is registered.

## Architecture

```text
GitHub webhook → Webhook server → Temporal → Go worker → build pods
                       ↑                         ├── GitHub reporting
                SvelteKit dashboard              ├── S3 logs/artifacts
                                                 └── experimental EKS workflows
```

| Path | Purpose |
| --- | --- |
| `cmd/webhook`, `cmd/worker`, `cmd/validate` | Service and validation entrypoints |
| `internal/workflows` | Workflow orchestration and retry/cancellation policies |
| `internal/activities`, `internal/k8s` | Checkout, execution, reporting, and pod management |
| `ui` | SvelteKit dashboard |
| `site` | Astro documentation site |
| `deploy` | Experimental Helm, Terraform, and Argo CD configuration |

## Deployment status

The deployment files are examples from the POC, not a supported production recipe. They need environment-specific configuration and review. Terraform, bootstrap scripts, and Helm installs can create billable resources.

The repository's existing main-branch Docker workflow can push to ECR and update GitOps configuration when enabled. The site workflow publishes GitHub Pages. Use a branch and pull request for changes; local checks and the PR CI workflow do not deploy either.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, validation, and contribution scope. Useful contributions include regression tests around restart recovery, workspace persistence, credentials in workflow history, and cluster cleanup.

## License

This repository does not yet include a license file.

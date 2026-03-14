# TemporalCI — Roadmap

## What's Been Built (Month 0)

39 commits, 1104 lines of Go, fully operational on EKS.

### Core CI Engine
- [x] GitHub webhook → Temporal workflow → clone → build/test/vet → report
- [x] `.temporalci.yaml` pipeline config loaded from each repo
- [x] Parallel step execution (no deps) and DAG-based sequential (with `depends_on`)
- [x] Per-step timeout from config
- [x] Cancel previous runs when new push arrives on same branch
- [x] Cancelled/skipped step detection and reporting

### GitHub Integration
- [x] Pending → success/failure commit status with step counts
- [x] PR comments with per-step timing, pass/fail summary, collapsible logs
- [x] 🔗 Direct link to Temporal Web UI workflow page in PR comments and commit status
- [x] PR action filtering (only `opened` and `synchronize`)

### Infrastructure & GitOps
- [x] EKS cluster with managed node groups, EBS CSI, pod identity
- [x] ArgoCD auto-sync from Git
- [x] Terraform for all infrastructure
- [x] `scripts/bootstrap.sh` — single command to create everything
- [x] Temporal Web UI exposed via LoadBalancer
- [x] Docker build via GitHub Actions → ECR → ArgoCD deploys

### Reliability
- [x] Disconnected context for reporting (survives workflow cancellation)
- [x] Output truncation (4KB) to prevent payload bloat
- [x] Cleanup activity removes clone dirs
- [x] Workflow query handler for real-time status

---

## Month 1: K8s-Native Execution & Caching

**Theme**: Stop running CI steps in the worker process. Run them as isolated K8s pods with proper resource limits, caching, and artifact support.

### Week 1-2: K8s Pod Execution (Production)

The K8s pod runner (`internal/k8s/pod.go`) exists but isn't wired up in production. Wire it up.

- [ ] **Enable K8s client in worker** — inject in-cluster client when running on EKS
- [ ] **CI node pool** — dedicated `ci-jobs` node group with taints so CI pods don't compete with Temporal
- [ ] **Resource limits from config** — `.temporalci.yaml` gets `resources` field per step:
  ```yaml
  steps:
    - name: build
      image: golang:1.23
      command: go build ./...
      resources:
        cpu: "1"
        memory: 2Gi
  ```
- [ ] **Pod log streaming** — stream pod logs to S3, link in PR comment
- [ ] **Pod cleanup** — garbage collect completed CI pods after 1 hour

### Week 3-4: Caching & Artifacts

- [ ] **Go module cache** — PVC-backed cache mounted into CI pods at `/go/pkg/mod`
- [ ] **Docker layer cache** — shared BuildKit cache for Docker-in-Docker steps
- [ ] **Artifact passing** — steps can declare `outputs` that are mounted into dependent steps:
  ```yaml
  steps:
    - name: build
      command: go build -o /artifacts/app ./...
      outputs: [/artifacts/app]
    - name: integration-test
      command: /artifacts/app --self-test
      depends_on: [build]
  ```
- [ ] **Cache key invalidation** — hash `go.sum` / `package-lock.json` for cache busting

---

## Month 2: Multi-Repo, Secrets & Environments

**Theme**: Make TemporalCI usable by any team. Support multiple repos, secret injection, and environment-scoped pipelines.

### Week 1-2: Multi-Repo & Self-Service

- [ ] **Repo registration API** — `POST /repos` to register a new repo + auto-create GitHub webhook
- [ ] **Per-repo config** — store repo settings in PostgreSQL (default branch, notification prefs)
- [ ] **GitHub App** — replace PAT with a proper GitHub App for:
  - Check Runs API (richer than commit status)
  - Installation-scoped tokens (no personal token needed)
  - Automatic webhook management
- [ ] **Org-wide installation** — install once, works for all repos in the org

### Week 3-4: Secrets & Environments

- [ ] **Secret injection** — steps can reference secrets from AWS Secrets Manager:
  ```yaml
  steps:
    - name: deploy
      command: helm upgrade ...
      secrets:
        - AWS_ACCESS_KEY_ID
        - DOCKER_PASSWORD
  ```
- [ ] **Environment pipelines** — `.temporalci.yaml` supports `on` triggers:
  ```yaml
  on:
    push:
      branches: [main]
    pull_request:
      branches: [main]
  
  environments:
    staging:
      on: { push: { branches: [main] } }
      steps:
        - name: deploy-staging
          type: helm-deploy
          chart: ./deploy/helm
          cluster: staging-cluster
  ```
- [ ] **Manual approval gates** — Temporal signals for human approval before prod deploy
- [ ] **Slack/Teams notifications** — webhook on pipeline completion

---

## Month 3: On-Demand EKS Clusters for Helm Testing

**Theme**: The big one. Spin up ephemeral EKS clusters to test Helm charts in isolation. Warm pool for fast startup, auto-cleanup for cost control.

### Week 1-2: Cluster Pool Manager

- [ ] **ClusterPool workflow** — long-running Temporal workflow that manages a pool of EKS clusters:
  ```
  ClusterPool workflow
    ├── Maintains desired pool size (e.g., 2 warm clusters)
    ├── Provisions new clusters when pool is empty
    ├── Handles lease/release via Temporal signals
    └── Auto-deletes clusters after TTL (e.g., 4 hours)
  ```
- [ ] **ProvisionCluster activity** — creates EKS cluster via AWS SDK:
  - EKS Auto Mode (no node group management)
  - Pre-configured with EBS CSI, CoreDNS, kube-proxy
  - VPC from a shared pool or dedicated per cluster
  - ~10 min provision time (warm pool hides this)
- [ ] **Warm pool** — keep N clusters ready, replenish on lease:
  ```
  Pool: [cluster-a (warm), cluster-b (warm)]
  
  PR opens → LeaseCluster → gets cluster-a → pool replenishes cluster-c
  Tests finish → ReleaseCluster → cluster-a returned to pool or destroyed
  ```
- [ ] **Cost controls**:
  - Max pool size (default: 3)
  - Cluster TTL (default: 4 hours, max: 24 hours)
  - Auto-destroy on idle (no lease for 30 min)
  - Budget alerts via CloudWatch

### Week 3-4: Helm Test Pipeline

- [ ] **`helm-test` step type** in `.temporalci.yaml`:
  ```yaml
  steps:
    - name: build
      command: docker build -t myapp .
    
    - name: helm-test
      type: helm-test
      chart: ./deploy/helm
      values: ./deploy/helm/values-test.yaml
      test_command: helm test myapp --timeout 5m
      depends_on: [build]
      cluster:
        pool: default        # Use cluster from pool
        ttl: 2h              # Release after 2 hours
        addons: [prometheus]  # Pre-install addons
  ```
- [ ] **HelmTestPipeline workflow**:
  ```
  1. LeaseCluster (from pool or provision)
  2. Configure kubeconfig
  3. helm install <chart> --values <values>
  4. Wait for pods ready
  5. helm test <release> OR custom test command
  6. Collect results + logs
  7. helm uninstall
  8. ReleaseCluster (return to pool)
  9. Report results to PR
  ```
- [ ] **Test results in PR comments**:
  ```
  ## TemporalCI Results
  
  ✅ **build** (45.2s)
  ✅ **helm-test** (3m 12s) — cluster: pool-cluster-a
  
  ### Helm Test Results
  ✅ myapp-connection-test (2.1s)
  ✅ myapp-api-health (1.3s)
  ✅ myapp-migration-test (8.7s)
  
  🔗 View workflow | 📊 Cluster dashboard
  ```
- [ ] **Cluster dashboard** — web page showing:
  - Pool status (warm/leased/provisioning)
  - Active test runs per cluster
  - Cost per cluster and total spend
  - Cluster lifecycle timeline

---

## Future (Month 4+)

- [ ] **Matrix builds** — run steps across multiple Go/Node/Python versions
- [ ] **Monorepo support** — detect changed paths, only run affected pipelines
- [ ] **Build badges** — `![CI](https://temporalci.example.com/badge/repo/branch)`
- [ ] **Webhook replay** — re-run any past CI run from the dashboard
- [ ] **Cost attribution** — per-repo and per-team CI cost tracking
- [ ] **Multi-cloud clusters** — GKE/AKS cluster pools alongside EKS
- [ ] **Plugin system** — custom step types via container images
- [ ] **Scheduled pipelines** — cron-triggered CI runs (nightly builds)

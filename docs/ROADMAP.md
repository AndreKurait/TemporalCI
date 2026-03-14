# TemporalCI — 3-Month Roadmap

## Month 1: Production Hardening & Core Features

### Week 1-2: Reliability & Observability
- [x] Pending commit status ("running") set at workflow start
- [x] Step-level timing in PR comments
- [ ] Structured logging with correlation IDs
- [ ] Prometheus metrics (workflow duration, step pass/fail rates)
- [ ] Grafana dashboard for CI health

### Week 3-4: Pipeline Features
- [x] Parallel step execution support in `.temporalci.yaml`
- [x] Step dependencies (DAG-based execution)
- [ ] Artifact passing between steps
- [ ] Caching (Go module cache, Docker layer cache)
- [ ] Timeout per-step from `.temporalci.yaml`

## Month 2: K8s-Native Execution & Multi-Repo

### Week 1-2: K8s Pod Execution
- [ ] Run CI steps as K8s pods (not local shell)
- [ ] Pod templates with resource limits from `.temporalci.yaml`
- [ ] Log streaming from pods to S3
- [ ] Dedicated CI node pool with taints/tolerations

### Week 3-4: Multi-Repo & Environments
- [ ] Multi-repo webhook support (register any repo)
- [ ] Environment-scoped pipelines (dev/staging/prod)
- [ ] Helm test integration (deploy + test + teardown)
- [ ] Secret injection into CI steps via Pod Identity

## Month 3: On-Demand EKS Clusters for Helm Testing

### Week 1-2: Cluster Pool Management
- [ ] `ClusterPool` CRD or Temporal workflow for managing warm EKS clusters
- [ ] Provision clusters via Terraform/EKS API from Temporal activities
- [ ] Cluster lifecycle: provision → warm pool → lease → test → recycle
- [ ] Cost controls: auto-delete after TTL, max pool size

### Week 3-4: Helm Test Workflow
- [ ] `HelmTestPipeline` workflow: lease cluster → deploy chart → run tests → report → release cluster
- [ ] `.temporalci.yaml` support for `helm-test` step type
- [ ] Integration test results in PR comments
- [ ] Cluster provisioning from pre-baked AMIs for fast startup
- [ ] Dashboard showing cluster pool status and test history

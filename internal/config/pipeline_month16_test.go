package config

import (
	"testing"
)

// --- Month 16: Multi-arch Docker builds, registry auth, SBOM, pipeline DAG ---

func TestLoadPipelineConfig_DockerBuild(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "release", `steps:
  - name: build-images
    docker: true
    privileged: true
    command: |
      docker buildx create --use
      docker buildx build --platform linux/amd64,linux/arm64 --push -t myrepo/app:latest .
    timeout: 30m
    resources:
      cpu: "4"
      memory: 8Gi
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	step := cfg.Pipelines["release"].Steps[0]
	if !step.Docker {
		t.Error("expected docker=true")
	}
	if !step.Privileged {
		t.Error("expected privileged=true")
	}
	if step.Resources == nil || step.Resources.CPU != "4" {
		t.Error("expected resources.cpu=4")
	}
}

func TestLoadPipelineConfig_AWSRoleChaining(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "release", `steps:
  - name: assume-base
    command: echo "assumed base role"
    aws_role:
      arn: arn:aws:iam::123:role/base
      duration: 3600
  - name: upload
    command: aws s3 cp artifact.tar.gz s3://bucket/
    depends_on: [assume-base]
    aws_role:
      arn: arn:aws:iam::123:role/upload
      source_credentials: assume-base
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["release"]
	base := p.Steps[0]
	if base.AWSRole == nil || base.AWSRole.ARN != "arn:aws:iam::123:role/base" {
		t.Error("expected base AWS role")
	}
	upload := p.Steps[1]
	if upload.AWSRole == nil || upload.AWSRole.SourceCredentials != "assume-base" {
		t.Error("expected chained credentials from assume-base")
	}
}

func TestLoadPipelineConfig_TriggerStep(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: test
    command: go test ./...
  - name: deploy
    depends_on: [test]
    trigger:
      pipeline: deploy
      parameters:
        ENV: staging
      wait: true
      propagate_failure: false
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	step := cfg.Pipelines["ci"].Steps[1]
	if step.Trigger == nil {
		t.Fatal("expected trigger config")
	}
	if step.Trigger.Pipeline != "deploy" {
		t.Errorf("pipeline = %q", step.Trigger.Pipeline)
	}
	if step.Trigger.Parameters["ENV"] != "staging" {
		t.Errorf("params = %v", step.Trigger.Parameters)
	}
	if step.Trigger.Wait == nil || !*step.Trigger.Wait {
		t.Error("expected wait=true")
	}
	if step.Trigger.PropagateFailure == nil || *step.Trigger.PropagateFailure {
		t.Error("expected propagate_failure=false")
	}
}

func TestValidate_DAGDependencies(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: clone
    command: git clone
  - name: build
    depends_on: [clone]
    command: go build
  - name: test
    depends_on: [build]
    command: go test
  - name: lint
    depends_on: [clone]
    command: golangci-lint
  - name: gate
    type: gate
    depends_on: [build, test, lint]
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("valid DAG should have no errors: %v", errs)
	}
}

func TestValidate_DAGCycle(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: a
    depends_on: [c]
  - name: b
    depends_on: [a]
  - name: c
    depends_on: [b]
`)
	cfg, _ := LoadPipelineConfig(dir)
	errs := cfg.Validate()
	found := false
	for _, e := range errs {
		if containsStr(e, "circular") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected circular dependency error, got %v", errs)
	}
}

func TestLoadPipelineConfig_LockAndLockPool(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "deploy", `steps:
  - name: deploy-staging
    command: ./deploy.sh
    lock: staging-deploy
    lock_timeout: 15m
  - name: integ-test
    command: ./test.sh
    lock_pool:
      label: integ-clusters
      quantity: 1
    lock_timeout: 30m
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["deploy"]
	if p.Steps[0].Lock != "staging-deploy" {
		t.Errorf("lock = %q", p.Steps[0].Lock)
	}
	if p.Steps[0].LockTimeout != "15m" {
		t.Errorf("lock_timeout = %q", p.Steps[0].LockTimeout)
	}
	if p.Steps[1].LockPool == nil || p.Steps[1].LockPool.Label != "integ-clusters" {
		t.Error("expected lock_pool config")
	}
}

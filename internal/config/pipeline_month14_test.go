package config

import (
	"testing"
)

// --- Month 14: CodeQL, SonarQube pipeline templates, service containers in config ---

func TestLoadPipelineConfig_CodeQL(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "codeql", `on:
  schedule:
    - cron: "25 22 * * 0"
  push:
    branches: [main]
steps:
  - name: analyze
    image: temporalci/ci-codeql
    matrix:
      language: [java-kotlin, javascript-typescript, python]
    command: |
      codeql database create /tmp/db --language=${{ matrix.language }}
      codeql database analyze /tmp/db --format=sarif-latest --output=/tmp/results.sarif
    timeout: 30m
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["codeql"]
	if p == nil {
		t.Fatal("missing codeql pipeline")
	}
	if len(p.On.Schedule) != 1 {
		t.Error("expected 1 schedule")
	}
	if p.Steps[0].Matrix == nil {
		t.Fatal("expected matrix on analyze step")
	}
	langs := p.Steps[0].Matrix.Dimensions["language"]
	if len(langs) != 3 {
		t.Errorf("expected 3 languages, got %d", len(langs))
	}
	if p.Steps[0].Timeout != "30m" {
		t.Errorf("timeout = %q", p.Steps[0].Timeout)
	}
}

func TestLoadPipelineConfig_SonarQube(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "sonarqube", `steps:
  - name: sonar-scan
    image: temporalci/ci-base:opensearch-migrations
    services:
      - name: sonarqube
        image: sonarqube:25.10-community
        ports: [9000]
        health:
          cmd: "curl -s http://localhost:9000/api/system/health | grep GREEN"
          interval: 10s
          retries: 60
    command: sonar-scanner -Dsonar.host.url=http://localhost:9000
    secrets: [SONAR_TOKEN]
    timeout: 15m
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["sonarqube"]
	step := p.Steps[0]
	if len(step.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(step.Services))
	}
	svc := step.Services[0]
	if svc.Name != "sonarqube" {
		t.Errorf("service name = %q", svc.Name)
	}
	if svc.Health == nil {
		t.Fatal("expected health check")
	}
	if svc.Health.Retries != 60 {
		t.Errorf("retries = %d", svc.Health.Retries)
	}
	if len(step.Secrets) != 1 || step.Secrets[0] != "SONAR_TOKEN" {
		t.Errorf("secrets = %v", step.Secrets)
	}
}

func TestLoadPipelineConfig_GateStep(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: build
    command: go build ./...
  - name: test
    command: go test ./...
    depends_on: [build]
  - name: lint
    command: golangci-lint run
  - name: all-checks-pass
    type: gate
    depends_on: [build, test, lint]
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["ci"]
	gate := p.Steps[3]
	if gate.Type != "gate" {
		t.Errorf("type = %q, want gate", gate.Type)
	}
	if len(gate.DependsOn) != 3 {
		t.Errorf("depends_on = %v", gate.DependsOn)
	}
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("unexpected validation errors: %v", errs)
	}
}

func TestLoadPipelineConfig_PostSteps(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: test
    command: go test ./...
    post:
      - "codecov --flag unit"
      - "cleanup.sh"
post:
  always:
    - name: notify
      command: curl $SLACK_WEBHOOK
  on_failure:
    - name: debug
      command: cat /tmp/debug.log
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["ci"]
	if len(p.Steps[0].Post) != 2 {
		t.Errorf("step post = %v", p.Steps[0].Post)
	}
	if p.Post == nil {
		t.Fatal("pipeline post should not be nil")
	}
	if len(p.Post.Always) != 1 {
		t.Errorf("always = %d", len(p.Post.Always))
	}
	if len(p.Post.OnFailure) != 1 {
		t.Errorf("on_failure = %d", len(p.Post.OnFailure))
	}
}

func TestLoadPipelineConfig_ArtifactUploadDownload(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: build
    command: go build -o /out/app
    artifacts:
      upload:
        - path: /out/app
  - name: test
    command: ./app --test
    depends_on: [build]
    artifacts:
      download:
        - from_step: build
          path: /out/
`)
	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Pipelines["ci"]
	if p.Steps[0].Artifacts == nil || len(p.Steps[0].Artifacts.Upload) != 1 {
		t.Error("expected upload artifact on build step")
	}
	if p.Steps[1].Artifacts == nil || len(p.Steps[1].Artifacts.Download) != 1 {
		t.Error("expected download artifact on test step")
	}
	if p.Steps[1].Artifacts.Download[0].FromStep != "build" {
		t.Errorf("from_step = %q", p.Steps[1].Artifacts.Download[0].FromStep)
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPipelineConfig(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: build
    image: golang:1.23
    command: go build ./...
    timeout: 5m
  - name: test
    image: golang:1.23
    command: go test -v ./...
    depends_on: [build]
    resources:
      cpu: "2"
      memory: 4Gi
`
	if err := os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(cfg.Steps))
	}
	if cfg.Steps[0].Timeout != "5m" {
		t.Errorf("step 0 timeout = %q, want 5m", cfg.Steps[0].Timeout)
	}
	if cfg.Steps[1].Resources == nil || cfg.Steps[1].Resources.CPU != "2" {
		t.Errorf("step 1 resources.cpu = %v, want 2", cfg.Steps[1].Resources)
	}
}

func TestLoadPipelineConfig_Missing(t *testing.T) {
	_, err := LoadPipelineConfig(t.TempDir())
	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Steps) != 2 {
		t.Fatalf("expected 2 default steps, got %d", len(cfg.Steps))
	}
}

func TestShouldRun_NoFilter(t *testing.T) {
	cfg := &PipelineConfig{}
	if !cfg.ShouldRun("push", "main") {
		t.Error("should run when no filter")
	}
}

func TestShouldRun_PushFilter(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			Push: &BranchFilter{Branches: []string{"main", "develop"}},
		},
	}
	if !cfg.ShouldRun("push", "main") {
		t.Error("should run for push to main")
	}
	if cfg.ShouldRun("push", "feature/x") {
		t.Error("should not run for push to feature/x")
	}
	if cfg.ShouldRun("pull_request", "main") {
		t.Error("should not run for PR when only push configured")
	}
}

func TestShouldRun_PRFilter(t *testing.T) {
	cfg := &PipelineConfig{
		On: &TriggerConfig{
			PullRequest: &BranchFilter{Branches: []string{"main"}},
		},
	}
	if !cfg.ShouldRun("pull_request", "main") {
		t.Error("should run for PR to main")
	}
}

func TestMatchingEnvironments(t *testing.T) {
	cfg := &PipelineConfig{
		Environments: map[string]*EnvConfig{
			"staging": {
				On: &TriggerConfig{Push: &BranchFilter{Branches: []string{"main"}}},
				Steps: []StepConfig{{Name: "deploy-staging", Command: "deploy"}},
			},
			"production": {
				On:       &TriggerConfig{Push: &BranchFilter{Branches: []string{"release"}}},
				Approval: true,
				Steps:    []StepConfig{{Name: "deploy-prod", Command: "deploy"}},
			},
		},
	}

	envs := cfg.MatchingEnvironments("push", "main")
	if len(envs) != 1 {
		t.Fatalf("expected 1 matching env, got %d", len(envs))
	}
	if _, ok := envs["staging"]; !ok {
		t.Error("staging should match push to main")
	}

	envs = cfg.MatchingEnvironments("push", "release")
	if _, ok := envs["production"]; !ok {
		t.Error("production should match push to release")
	}
}

func TestPipelineConfig_Secrets(t *testing.T) {
	dir := t.TempDir()
	yaml := `steps:
  - name: deploy
    image: alpine
    command: deploy.sh
    secrets:
      - DOCKER_PASSWORD
      - NPM_TOKEN
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Steps[0].Secrets) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(cfg.Steps[0].Secrets))
	}
	if cfg.Steps[0].Secrets[0] != "DOCKER_PASSWORD" {
		t.Errorf("secret[0] = %q, want DOCKER_PASSWORD", cfg.Steps[0].Secrets[0])
	}
}

func TestPipelineConfig_FullYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

steps:
  - name: build
    image: golang:1.23
    command: go build ./...
  - name: test
    image: golang:1.23
    command: go test ./...
    depends_on: [build]

environments:
  staging:
    on:
      push:
        branches: [main]
    steps:
      - name: deploy-staging
        image: alpine
        command: helm upgrade staging
        secrets: [KUBE_TOKEN]
  production:
    on:
      push:
        branches: [release]
    approval: true
    steps:
      - name: deploy-prod
        image: alpine
        command: helm upgrade prod
`
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(yaml), 0644)

	cfg, err := LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ShouldRun("push", "main") {
		t.Error("should run for push to main")
	}
	if len(cfg.Environments) != 2 {
		t.Errorf("expected 2 environments, got %d", len(cfg.Environments))
	}
	if !cfg.Environments["production"].Approval {
		t.Error("production should require approval")
	}
}

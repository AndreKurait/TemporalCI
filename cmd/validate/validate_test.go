package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AndreKurait/TemporalCI/internal/config"
)

func writePipeline(t *testing.T, dir, name, content string) {
	t.Helper()
	pDir := filepath.Join(dir, ".temporalci")
	os.MkdirAll(pDir, 0755)
	os.WriteFile(filepath.Join(pDir, name+".yaml"), []byte(content), 0644)
}

func TestValidate_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: build
    image: golang:1.24
    command: go build ./...
  - name: test
    depends_on: [build]
    command: go test ./...
`)

	cfg, err := config.LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_CircularDeps(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: a
    depends_on: [b]
    command: echo a
  - name: b
    depends_on: [a]
    command: echo b
`)

	cfg, err := config.LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Error("expected circular dependency error")
	}
}

func TestValidate_UnknownDep(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: test
    depends_on: [nonexistent]
    command: echo test
`)

	cfg, err := config.LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Error("expected unknown dep error")
	}
}

func TestValidate_DuplicateStepName(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: build
    command: echo 1
  - name: build
    command: echo 2
`)

	cfg, err := config.LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Error("expected duplicate step name error")
	}
}

func TestValidate_EmptyMatrixAxis(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: test
    matrix:
      index: []
    command: echo $MATRIX_INDEX
`)

	cfg, err := config.LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := cfg.Validate()
	if len(errs) == 0 {
		t.Error("expected empty matrix axis error")
	}
}

func TestValidate_MultiPipeline(t *testing.T) {
	dir := t.TempDir()
	writePipeline(t, dir, "ci", `steps:
  - name: test
    command: go test
`)
	writePipeline(t, dir, "deploy", `parameters:
  - name: ENV
    type: choice
    options: [staging, prod]
steps:
  - name: deploy
    command: ./deploy.sh
`)

	cfg, err := config.LoadPipelineConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	pipelines := cfg.GetPipelines()
	if len(pipelines) != 2 {
		t.Errorf("expected 2 pipelines, got %d", len(pipelines))
	}
}

func TestValidate_MissingConfig(t *testing.T) {
	_, err := config.LoadPipelineConfig(t.TempDir())
	if err == nil {
		t.Error("expected error for missing config")
	}
}

func TestValidate_SelfHostingConfig(t *testing.T) {
	cfg, err := config.LoadPipelineConfig("../..")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("self-hosting config has errors: %v", errs)
	}
}

func TestValidate_OpensearchExample(t *testing.T) {
	cfg, err := config.LoadPipelineConfig("../../examples/opensearch-migrations")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("opensearch-migrations config has errors: %v", errs)
	}
	pipelines := cfg.GetPipelines()
	if _, ok := pipelines["ci"]; !ok {
		t.Error("missing ci pipeline")
	}
	if _, ok := pipelines["release"]; !ok {
		t.Error("missing release pipeline")
	}
}

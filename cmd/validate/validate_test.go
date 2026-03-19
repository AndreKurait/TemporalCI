package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AndreKurait/TemporalCI/internal/config"
)

func TestValidate_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(`steps:
  - name: build
    image: golang:1.24
    command: go build ./...
  - name: test
    depends_on: [build]
    command: go test ./...
`), 0644)

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
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(`steps:
  - name: a
    depends_on: [b]
    command: echo a
  - name: b
    depends_on: [a]
    command: echo b
`), 0644)

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
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(`steps:
  - name: test
    depends_on: [nonexistent]
    command: echo test
`), 0644)

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
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(`steps:
  - name: build
    command: echo 1
  - name: build
    command: echo 2
`), 0644)

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
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(`steps:
  - name: test
    matrix:
      index: []
    command: echo $MATRIX_INDEX
`), 0644)

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
	os.WriteFile(filepath.Join(dir, ".temporalci.yaml"), []byte(`pipelines:
  ci:
    steps:
      - name: test
        command: go test
  deploy:
    parameters:
      - name: ENV
        type: choice
        options: [staging, prod]
    steps:
      - name: deploy
        command: ./deploy.sh
`), 0644)

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
	// Validate the actual .temporalci.yaml in the project root
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

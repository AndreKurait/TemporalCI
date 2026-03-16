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
	if cfg.Steps[0].Name != "build" {
		t.Errorf("step 0 name = %q, want build", cfg.Steps[0].Name)
	}
	if cfg.Steps[0].Timeout != "5m" {
		t.Errorf("step 0 timeout = %q, want 5m", cfg.Steps[0].Timeout)
	}
	if cfg.Steps[1].DependsOn[0] != "build" {
		t.Errorf("step 1 depends_on = %v, want [build]", cfg.Steps[1].DependsOn)
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
	if cfg.Steps[0].Name != "build" || cfg.Steps[1].Name != "test" {
		t.Error("unexpected default step names")
	}
}

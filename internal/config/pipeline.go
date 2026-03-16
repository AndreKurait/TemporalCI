package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PipelineConfig defines the CI pipeline from .temporalci.yaml.
type PipelineConfig struct {
	Steps []StepConfig `yaml:"steps"`
}

// StepConfig defines a single pipeline step.
type StepConfig struct {
	Name      string          `yaml:"name"`
	Image     string          `yaml:"image"`
	Command   string          `yaml:"command"`
	Timeout   string          `yaml:"timeout,omitempty"`
	DependsOn []string        `yaml:"depends_on,omitempty"`
	Resources *ResourceConfig `yaml:"resources,omitempty"`
}

// ResourceConfig defines resource limits for a step.
type ResourceConfig struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// LoadPipelineConfig reads .temporalci.yaml from the given directory.
func LoadPipelineConfig(dir string) (*PipelineConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".temporalci.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read pipeline config: %w", err)
	}
	var cfg PipelineConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse pipeline config: %w", err)
	}
	return &cfg, nil
}

// DefaultConfig returns a default Go build+test pipeline.
func DefaultConfig() *PipelineConfig {
	return &PipelineConfig{
		Steps: []StepConfig{
			{Name: "build", Image: "golang:1.23", Command: "go build ./..."},
			{Name: "test", Image: "golang:1.23", Command: "go test ./..."},
		},
	}
}

package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PipelineConfig defines the CI pipeline configuration from .temporalci.yaml.
type PipelineConfig struct {
	On           *TriggerConfig          `yaml:"on,omitempty"`
	Environments map[string]Environment  `yaml:"environments,omitempty"`
	Steps        []StepConfig            `yaml:"steps"`
}

// TriggerConfig defines when pipelines run.
type TriggerConfig struct {
	Push        *BranchFilter `yaml:"push,omitempty"`
	PullRequest *BranchFilter `yaml:"pull_request,omitempty"`
}

// BranchFilter filters by branch names.
type BranchFilter struct {
	Branches []string `yaml:"branches,omitempty"`
}

// Environment defines an environment-specific pipeline.
type Environment struct {
	On    TriggerConfig `yaml:"on"`
	Steps []StepConfig  `yaml:"steps"`
}

// StepConfig defines a single pipeline step.
type StepConfig struct {
	Name      string            `yaml:"name"`
	Image     string            `yaml:"image"`
	Command   string            `yaml:"command"`
	Timeout   string            `yaml:"timeout,omitempty"`
	DependsOn []string          `yaml:"depends_on,omitempty"`
	Type      string            `yaml:"type,omitempty"` // "run" (default), "helm-test"
	Resources *ResourceConfig   `yaml:"resources,omitempty"`
	Secrets   []string          `yaml:"secrets,omitempty"`
	Outputs   []string          `yaml:"outputs,omitempty"`
	Helm      *HelmTestConfig   `yaml:"helm,omitempty"`
}

// ResourceConfig defines resource limits for a step.
type ResourceConfig struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

// HelmTestConfig defines configuration for helm-test step type.
type HelmTestConfig struct {
	Chart       string `yaml:"chart"`
	Values      string `yaml:"values,omitempty"`
	TestCommand string `yaml:"test_command,omitempty"`
	ClusterPool string `yaml:"cluster_pool,omitempty"`
	ClusterTTL  string `yaml:"cluster_ttl,omitempty"`
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

// ShouldRun checks if the pipeline should run for the given event and ref.
func (c *PipelineConfig) ShouldRun(event, ref string) bool {
	if c.On == nil {
		return true // no filter = always run
	}
	switch event {
	case "push":
		return c.On.Push == nil || matchBranch(c.On.Push.Branches, ref)
	case "pull_request":
		return c.On.PullRequest == nil || matchBranch(c.On.PullRequest.Branches, ref)
	}
	return true
}

func matchBranch(branches []string, ref string) bool {
	if len(branches) == 0 {
		return true
	}
	for _, b := range branches {
		if ref == b || ref == "refs/heads/"+b {
			return true
		}
	}
	return false
}

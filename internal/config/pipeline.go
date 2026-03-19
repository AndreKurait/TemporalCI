package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PipelineConfig defines the CI pipeline from .temporalci.yaml.
type PipelineConfig struct {
	On           *TriggerConfig          `yaml:"on,omitempty"`
	Steps        []StepConfig            `yaml:"steps"`
	Environments map[string]*EnvConfig   `yaml:"environments,omitempty"`
}

// TriggerConfig defines when the pipeline runs.
type TriggerConfig struct {
	Push        *BranchFilter `yaml:"push,omitempty"`
	PullRequest *BranchFilter `yaml:"pull_request,omitempty"`
}

// BranchFilter filters by branch names.
type BranchFilter struct {
	Branches []string `yaml:"branches,omitempty"`
}

// EnvConfig defines an environment-scoped pipeline.
type EnvConfig struct {
	On       *TriggerConfig `yaml:"on,omitempty"`
	Approval bool           `yaml:"approval,omitempty"` // Require manual approval
	Steps    []StepConfig   `yaml:"steps"`
}

// StepConfig defines a single pipeline step.
type StepConfig struct {
	Name      string          `yaml:"name"`
	Image     string          `yaml:"image"`
	Command   string          `yaml:"command"`
	Timeout   string          `yaml:"timeout,omitempty"`
	DependsOn []string        `yaml:"depends_on,omitempty"`
	Resources *ResourceConfig `yaml:"resources,omitempty"`
	Secrets   []string        `yaml:"secrets,omitempty"`
	HelmTest  *HelmTestConfig `yaml:"helm_test,omitempty"`
}

// HelmTestConfig defines a helm test step that runs on an ephemeral cluster.
type HelmTestConfig struct {
	ChartPath   string            `yaml:"chart_path"`
	ReleaseName string            `yaml:"release_name,omitempty"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Values      map[string]string `yaml:"values,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty"`
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

// ShouldRun checks if the pipeline should run for the given event and ref.
func (c *PipelineConfig) ShouldRun(event, ref string) bool {
	if c.On == nil {
		return true // no filter = always run
	}
	switch event {
	case "push":
		return c.On.Push != nil && matchBranch(c.On.Push.Branches, ref)
	case "pull_request":
		return c.On.PullRequest != nil && matchBranch(c.On.PullRequest.Branches, ref)
	}
	return true
}

// MatchingEnvironments returns environments that should trigger for the given event/ref.
func (c *PipelineConfig) MatchingEnvironments(event, ref string) map[string]*EnvConfig {
	result := make(map[string]*EnvConfig)
	for name, env := range c.Environments {
		if env.On == nil {
			continue
		}
		switch event {
		case "push":
			if env.On.Push != nil && matchBranch(env.On.Push.Branches, ref) {
				result[name] = env
			}
		case "pull_request":
			if env.On.PullRequest != nil && matchBranch(env.On.PullRequest.Branches, ref) {
				result[name] = env
			}
		}
	}
	return result
}

func matchBranch(branches []string, ref string) bool {
	if len(branches) == 0 {
		return true
	}
	for _, b := range branches {
		if b == ref {
			return true
		}
	}
	return false
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// PipelineConfig defines the CI pipeline from .temporalci.yaml.
type PipelineConfig struct {
	On           *TriggerConfig        `yaml:"on,omitempty"`
	Parameters   []ParameterConfig     `yaml:"parameters,omitempty"`
	Steps        []StepConfig          `yaml:"steps,omitempty"`
	Post         *PostConfig           `yaml:"post,omitempty"`
	Environments map[string]*EnvConfig `yaml:"environments,omitempty"`
	Pipelines    map[string]*Pipeline  `yaml:"pipelines,omitempty"`
}

// Pipeline defines a named pipeline within a multi-pipeline config.
type Pipeline struct {
	On         *TriggerConfig    `yaml:"on,omitempty"`
	Parameters []ParameterConfig `yaml:"parameters,omitempty"`
	Steps      []StepConfig      `yaml:"steps,omitempty"`
	Post       *PostConfig       `yaml:"post,omitempty"`
	Matrix     *MatrixConfig     `yaml:"matrix,omitempty"`
	Timeout    string            `yaml:"timeout,omitempty"`
}

// TriggerConfig defines when the pipeline runs.
type TriggerConfig struct {
	Push        *PushFilter     `yaml:"push,omitempty"`
	PullRequest *BranchFilter   `yaml:"pull_request,omitempty"`
	Schedule    []ScheduleEntry `yaml:"schedule,omitempty"`
	Release     *ReleaseFilter  `yaml:"release,omitempty"`
	Webhook     *WebhookMatch   `yaml:"webhook,omitempty"`
	Issues      *EventFilter    `yaml:"issues,omitempty"`
}

// PushFilter filters by branch and tag names.
type PushFilter struct {
	Branches []string `yaml:"branches,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
}

// BranchFilter filters by branch names.
type BranchFilter struct {
	Branches []string `yaml:"branches,omitempty"`
	Labels   []string `yaml:"labels,omitempty"`
}

// ScheduleEntry defines a cron schedule.
type ScheduleEntry struct {
	Cron string `yaml:"cron"`
}

// ReleaseFilter filters release events.
type ReleaseFilter struct {
	Types []string `yaml:"types,omitempty"` // published, created, etc.
}

// WebhookMatch matches custom webhook payloads.
type WebhookMatch struct {
	Match map[string]string `yaml:"match,omitempty"`
}

// EventFilter filters issue/PR lifecycle events.
type EventFilter struct {
	Types []string `yaml:"types,omitempty"`
}

// ParameterConfig defines a pipeline parameter.
type ParameterConfig struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`                  // string, choice, boolean
	Default     string   `yaml:"default,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Options     []string `yaml:"options,omitempty"`      // for choice type
}

// PostConfig defines cleanup/notification steps.
type PostConfig struct {
	Always    []StepConfig `yaml:"always,omitempty"`
	OnFailure []StepConfig `yaml:"on_failure,omitempty"`
}

// MatrixConfig defines matrix build dimensions.
type MatrixConfig struct {
	Dimensions  map[string][]string `yaml:",inline"`
	Exclude     []map[string]string `yaml:"exclude,omitempty"`
	Include     []map[string]string `yaml:"include,omitempty"`
	FailFast    *bool               `yaml:"fail_fast,omitempty"`
	MaxParallel int                 `yaml:"max_parallel,omitempty"`
}

// StepConfig defines a single pipeline step.
type StepConfig struct {
	Name       string          `yaml:"name"`
	Image      string          `yaml:"image,omitempty"`
	Command    string          `yaml:"command,omitempty"`
	Commands   []string        `yaml:"commands,omitempty"`
	Timeout    string          `yaml:"timeout,omitempty"`
	DependsOn  []string        `yaml:"depends_on,omitempty"`
	Resources  *ResourceConfig `yaml:"resources,omitempty"`
	Secrets    []string        `yaml:"secrets,omitempty"`
	HelmTest   *HelmTestConfig `yaml:"helm_test,omitempty"`
	When       string          `yaml:"when,omitempty"`
	Type       string          `yaml:"type,omitempty"` // "", "gate"
	Matrix     *MatrixConfig   `yaml:"matrix,omitempty"`
	Services   []ServiceConfig `yaml:"services,omitempty"`
	Docker     bool            `yaml:"docker,omitempty"`
	Privileged bool            `yaml:"privileged,omitempty"`
	Artifacts  *ArtifactConfig `yaml:"artifacts,omitempty"`
	Lock       string          `yaml:"lock,omitempty"`
	LockPool   *LockPoolRef    `yaml:"lock_pool,omitempty"`
	LockTimeout string         `yaml:"lock_timeout,omitempty"`
	AWSRole    *AWSRoleConfig  `yaml:"aws_role,omitempty"`
	Trigger    *TriggerStep    `yaml:"trigger,omitempty"`
	AllowSkip  bool            `yaml:"allow-skip,omitempty"`
}

// ServiceConfig defines a sidecar service container.
type ServiceConfig struct {
	Name   string            `yaml:"name"`
	Image  string            `yaml:"image"`
	Ports  []int             `yaml:"ports,omitempty"`
	Health *HealthCheck      `yaml:"health,omitempty"`
	Env    map[string]string `yaml:"env,omitempty"`
}

// HealthCheck defines a service health check.
type HealthCheck struct {
	Cmd      string `yaml:"cmd"`
	Interval string `yaml:"interval,omitempty"`
	Retries  int    `yaml:"retries,omitempty"`
}

// ArtifactConfig defines artifact upload/download for a step.
type ArtifactConfig struct {
	Upload   []ArtifactUpload   `yaml:"upload,omitempty"`
	Download []ArtifactDownload `yaml:"download,omitempty"`
}

// ArtifactUpload defines a path to upload as an artifact.
type ArtifactUpload struct {
	Path string `yaml:"path"`
}

// ArtifactDownload defines an artifact to download from a previous step.
type ArtifactDownload struct {
	FromStep     string `yaml:"from_step,omitempty"`
	FromPipeline string `yaml:"from_pipeline,omitempty"`
	Path         string `yaml:"path"`
}

// LockPoolRef references a lock pool.
type LockPoolRef struct {
	Label    string `yaml:"label"`
	Quantity int    `yaml:"quantity"`
}

// AWSRoleConfig defines IAM role assumption for a step.
type AWSRoleConfig struct {
	ARN         string `yaml:"arn"`
	Duration    int    `yaml:"duration,omitempty"`
	SessionName string `yaml:"session_name,omitempty"`
}

// TriggerStep triggers a child pipeline.
type TriggerStep struct {
	Pipeline         string            `yaml:"pipeline"`
	Parameters       map[string]string `yaml:"parameters,omitempty"`
	Wait             *bool             `yaml:"wait,omitempty"`
	PropagateFailure *bool             `yaml:"propagate_failure,omitempty"`
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

// EnvConfig defines an environment-scoped pipeline.
type EnvConfig struct {
	On       *TriggerConfig `yaml:"on,omitempty"`
	Approval bool           `yaml:"approval,omitempty"`
	Steps    []StepConfig   `yaml:"steps"`
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
			{Name: "build", Image: "golang:1.26", Command: "go build ./..."},
			{Name: "test", Image: "golang:1.26", Command: "go test ./..."},
		},
	}
}

// GetPipelines returns all named pipelines. If the config uses the flat format,
// it wraps it as a single pipeline named "default".
func (c *PipelineConfig) GetPipelines() map[string]*Pipeline {
	if len(c.Pipelines) > 0 {
		return c.Pipelines
	}
	return map[string]*Pipeline{
		"default": {
			On:         c.On,
			Parameters: c.Parameters,
			Steps:      c.Steps,
			Post:       c.Post,
		},
	}
}

// ResolveParameters merges parameter defaults with overrides and returns env vars.
func ResolveParameters(params []ParameterConfig, overrides map[string]string) (map[string]string, error) {
	env := make(map[string]string, len(params))
	for _, p := range params {
		val := p.Default
		if v, ok := overrides[p.Name]; ok {
			val = v
		}
		if p.Type == "choice" && len(p.Options) > 0 && val != "" {
			valid := false
			for _, opt := range p.Options {
				if opt == val {
					valid = true
					break
				}
			}
			if !valid {
				return nil, fmt.Errorf("parameter %q: value %q not in options %v", p.Name, val, p.Options)
			}
		}
		if p.Type == "boolean" && val != "" && val != "true" && val != "false" {
			return nil, fmt.Errorf("parameter %q: value %q is not a boolean", p.Name, val)
		}
		env[p.Name] = val
	}
	return env, nil
}

// Validate checks the pipeline config for errors.
func (c *PipelineConfig) Validate() []string {
	var errs []string
	for name, p := range c.GetPipelines() {
		errs = append(errs, validatePipeline(name, p)...)
	}
	return errs
}

func validatePipeline(name string, p *Pipeline) []string {
	var errs []string
	prefix := ""
	if name != "default" {
		prefix = fmt.Sprintf("pipeline %q: ", name)
	}

	stepNames := map[string]bool{}
	for _, s := range p.Steps {
		if s.Name == "" {
			errs = append(errs, prefix+"step missing name")
			continue
		}
		if stepNames[s.Name] {
			errs = append(errs, fmt.Sprintf("%sduplicate step name %q", prefix, s.Name))
		}
		stepNames[s.Name] = true
	}

	// Check depends_on references
	for _, s := range p.Steps {
		for _, dep := range s.DependsOn {
			if !stepNames[dep] {
				errs = append(errs, fmt.Sprintf("%sstep %q depends on unknown step %q", prefix, s.Name, dep))
			}
		}
	}

	// Check circular dependencies
	if cycle := detectCycle(p.Steps); cycle != "" {
		errs = append(errs, prefix+"circular dependency: "+cycle)
	}

	// Check parameters
	for _, param := range p.Parameters {
		if param.Type == "choice" && len(param.Options) == 0 {
			errs = append(errs, fmt.Sprintf("%sparameter %q: choice type requires options", prefix, param.Name))
		}
	}

	// Check matrix
	if p.Matrix != nil {
		for k, v := range p.Matrix.Dimensions {
			if k == "exclude" || k == "include" || k == "fail_fast" || k == "max_parallel" {
				continue
			}
			if len(v) == 0 {
				errs = append(errs, fmt.Sprintf("%smatrix axis %q is empty", prefix, k))
			}
		}
	}
	for _, s := range p.Steps {
		if s.Matrix != nil {
			for k, v := range s.Matrix.Dimensions {
				if k == "exclude" || k == "include" || k == "fail_fast" || k == "max_parallel" {
					continue
				}
				if len(v) == 0 {
					errs = append(errs, fmt.Sprintf("%sstep %q: matrix axis %q is empty", prefix, s.Name, k))
				}
			}
		}
	}

	return errs
}

func detectCycle(steps []StepConfig) string {
	graph := map[string][]string{}
	for _, s := range steps {
		graph[s.Name] = s.DependsOn
	}

	visited := map[string]int{} // 0=unvisited, 1=visiting, 2=done
	var path []string

	var visit func(string) bool
	visit = func(name string) bool {
		if visited[name] == 2 {
			return false
		}
		if visited[name] == 1 {
			return true
		}
		visited[name] = 1
		path = append(path, name)
		for _, dep := range graph[name] {
			if visit(dep) {
				return true
			}
		}
		path = path[:len(path)-1]
		visited[name] = 2
		return false
	}

	for _, s := range steps {
		if visit(s.Name) {
			return strings.Join(path, " → ")
		}
	}
	return ""
}

// ShouldRun checks if the pipeline should run for the given event and ref.
func (c *PipelineConfig) ShouldRun(event, ref string) bool {
	if c.On == nil {
		return true
	}
	return triggerMatches(c.On, event, ref)
}

// MatchingEnvironments returns environments that should trigger for the given event/ref.
func (c *PipelineConfig) MatchingEnvironments(event, ref string) map[string]*EnvConfig {
	result := make(map[string]*EnvConfig)
	for name, env := range c.Environments {
		if env.On == nil {
			continue
		}
		if triggerMatches(env.On, event, ref) {
			result[name] = env
		}
	}
	return result
}

func triggerMatches(tc *TriggerConfig, event, ref string) bool {
	switch event {
	case "push":
		if tc.Push != nil {
			return matchBranch(tc.Push.Branches, ref) || matchTag(tc.Push.Tags, ref)
		}
	case "pull_request":
		if tc.PullRequest != nil {
			return matchBranch(tc.PullRequest.Branches, ref)
		}
	case "release":
		return tc.Release != nil
	case "schedule":
		return len(tc.Schedule) > 0
	}
	return false
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

func matchTag(tags []string, ref string) bool {
	if len(tags) == 0 {
		return false
	}
	for _, t := range tags {
		if t == "*" || t == ref {
			return true
		}
	}
	return false
}

// GetCommand returns the effective command for a step (Command or joined Commands).
func (s *StepConfig) GetCommand() string {
	if s.Command != "" {
		return s.Command
	}
	if len(s.Commands) > 0 {
		return strings.Join(s.Commands, " && ")
	}
	return ""
}

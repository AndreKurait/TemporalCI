package activities

import "strings"

// CloneInput defines the input for the CloneRepo activity.
type CloneInput struct {
	Repo         string `json:"repo"`
	Ref          string `json:"ref"`
	WorkflowID   string `json:"workflowID"`
	PipelineName string `json:"pipelineName,omitempty"`
}

// CloneResult defines the output of the CloneRepo activity.
type CloneResult struct {
	Dir       string       `json:"dir"`
	Steps     []StepConfig `json:"steps,omitempty"`
	Post      *PostConfig  `json:"post,omitempty"`
	Pipelines []string     `json:"pipelines,omitempty"`
}

// StepConfig mirrors config.StepConfig for serialization across activity boundary.
type StepConfig struct {
	Name       string          `json:"name"`
	Image      string          `json:"image"`
	Command    string          `json:"command"`
	Commands   []string        `json:"commands,omitempty"`
	Timeout    string          `json:"timeout,omitempty"`
	DependsOn  []string        `json:"dependsOn,omitempty"`
	Resources  *ResourceConfig `json:"resources,omitempty"`
	Secrets    []string        `json:"secrets,omitempty"`
	HelmTest   *HelmTestConfig `json:"helmTest,omitempty"`
	When       string          `json:"when,omitempty"`
	Type       string          `json:"type,omitempty"`
	Matrix     *MatrixConfig   `json:"matrix,omitempty"`
	Services   []ServiceConfig `json:"services,omitempty"`
	Docker     bool            `json:"docker,omitempty"`
	Privileged bool            `json:"privileged,omitempty"`
	Artifacts  *ArtifactConfig `json:"artifacts,omitempty"`
	Lock       string          `json:"lock,omitempty"`
	LockPool   *LockPoolRef    `json:"lockPool,omitempty"`
	LockTimeout string         `json:"lockTimeout,omitempty"`
	AWSRole    *AWSRoleConfig  `json:"awsRole,omitempty"`
	Trigger    *TriggerStep    `json:"trigger,omitempty"`
	AllowSkip  bool            `json:"allowSkip,omitempty"`
	Outputs    map[string]string `json:"outputs,omitempty"`
}

// GetEffectiveCommand returns the command string, joining Commands if needed.
func (s *StepConfig) GetEffectiveCommand() string {
	if s.Command != "" {
		return s.Command
	}
	if len(s.Commands) > 0 {
		return strings.Join(s.Commands, " && ")
	}
	return ""
}

// MatrixConfig defines matrix build dimensions.
type MatrixConfig struct {
	Dimensions  map[string][]string `json:"dimensions"`
	Exclude     []map[string]string `json:"exclude,omitempty"`
	Include     []map[string]string `json:"include,omitempty"`
	FailFast    bool                `json:"failFast,omitempty"`
	MaxParallel int                 `json:"maxParallel,omitempty"`
}

// ServiceConfig defines a sidecar service container.
type ServiceConfig struct {
	Name   string            `json:"name"`
	Image  string            `json:"image"`
	Ports  []int             `json:"ports,omitempty"`
	Health *HealthCheck      `json:"health,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
}

// HealthCheck defines a service health check.
type HealthCheck struct {
	Cmd      string `json:"cmd"`
	Interval string `json:"interval,omitempty"`
	Retries  int    `json:"retries,omitempty"`
}

// ArtifactConfig defines artifact upload/download for a step.
type ArtifactConfig struct {
	Upload   []ArtifactUpload   `json:"upload,omitempty"`
	Download []ArtifactDownload `json:"download,omitempty"`
}

// ArtifactUpload defines a path to upload as an artifact.
type ArtifactUpload struct {
	Path string `json:"path"`
}

// ArtifactDownload defines an artifact to download from a previous step.
type ArtifactDownload struct {
	FromStep     string `json:"fromStep,omitempty"`
	FromPipeline string `json:"fromPipeline,omitempty"`
	Path         string `json:"path"`
}

// LockPoolRef references a lock pool.
type LockPoolRef struct {
	Label    string `json:"label"`
	Quantity int    `json:"quantity"`
}

// AWSRoleConfig defines IAM role assumption for a step.
type AWSRoleConfig struct {
	ARN         string `json:"arn"`
	Duration    int    `json:"duration,omitempty"`
	SessionName string `json:"sessionName,omitempty"`
}

// TriggerStep triggers a child pipeline.
type TriggerStep struct {
	Pipeline         string            `json:"pipeline"`
	Parameters       map[string]string `json:"parameters,omitempty"`
	Wait             bool              `json:"wait"`
	PropagateFailure bool              `json:"propagateFailure"`
}

// PostConfig defines cleanup/notification steps.
type PostConfig struct {
	Always    []StepConfig `json:"always,omitempty"`
	OnFailure []StepConfig `json:"onFailure,omitempty"`
}

// HelmTestConfig defines a helm test step.
type HelmTestConfig struct {
	ChartPath   string            `json:"chartPath"`
	ReleaseName string            `json:"releaseName,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	Values      map[string]string `json:"values,omitempty"`
	Timeout     string            `json:"timeout,omitempty"`
}

// ResourceConfig defines resource limits for a CI step pod.
type ResourceConfig struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// RunStepInput defines the input for the RunStep activity.
type RunStepInput struct {
	Dir              string             `json:"dir"`
	Command          string             `json:"command"`
	Name             string             `json:"name"`
	Image            string             `json:"image"`
	Resources        *ResourceConfig    `json:"resources,omitempty"`
	Repo             string             `json:"repo,omitempty"`
	Ref              string             `json:"ref,omitempty"`
	Secrets          map[string]string  `json:"secrets,omitempty"`
	Services         []ServiceConfig    `json:"services,omitempty"`
	Docker           bool               `json:"docker,omitempty"`
	Privileged       bool               `json:"privileged,omitempty"`
	CollectOutputs   bool               `json:"collectOutputs,omitempty"`
	AWSRole          *AWSRoleConfig     `json:"awsRole,omitempty"`
	ArtifactUpload   []ArtifactUpload   `json:"artifactUpload,omitempty"`
	ArtifactDownload []ArtifactDownload `json:"artifactDownload,omitempty"`
	MatrixVars       map[string]string  `json:"matrixVars,omitempty"`
}

// RunStepResult defines the output of the RunStep activity.
type RunStepResult struct {
	ExitCode     int               `json:"exitCode"`
	Output       string            `json:"output"`
	LogURL       string            `json:"logURL,omitempty"`
	Outputs      map[string]string `json:"outputs,omitempty"`
	ArtifactURLs []string          `json:"artifactURLs,omitempty"`
}

// StepResult captures the result of a single CI step (used in reporting).
type StepResult struct {
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	Output    string            `json:"output"`
	ExitCode  int               `json:"exitCode"`
	Duration  float64           `json:"duration"`
	LogURL    string            `json:"logURL,omitempty"`
	Outputs   map[string]string `json:"outputs,omitempty"`
	MatrixKey string            `json:"matrixKey,omitempty"`
}

// ReportInput defines the input for the ReportResults activity.
type ReportInput struct {
	Repo         string       `json:"repo"`
	HeadSHA      string       `json:"headSHA"`
	PRNumber     int          `json:"prNumber"`
	Steps        []StepResult `json:"steps"`
	WorkflowID   string       `json:"workflowID"`
	PipelineName string       `json:"pipelineName,omitempty"`
}

// StatusInput defines the input for the SetCommitStatus activity.
type StatusInput struct {
	Repo        string `json:"repo"`
	HeadSHA     string `json:"headSHA"`
	State       string `json:"state"`
	Description string `json:"description"`
}

// MatrixChildInput defines the input for a matrix child workflow.
type MatrixChildInput struct {
	ParentWorkflowID string            `json:"parentWorkflowID"`
	PipelineName     string            `json:"pipelineName"`
	StepName         string            `json:"stepName"`
	MatrixKey        string            `json:"matrixKey"`
	MatrixVars       map[string]string `json:"matrixVars"`
	Step             StepConfig        `json:"step"`
	Dir              string            `json:"dir"`
	Repo             string            `json:"repo"`
	Ref              string            `json:"ref"`
	HeadSHA          string            `json:"headSHA"`
	Secrets          map[string]string `json:"secrets,omitempty"`
}

// MatrixChildResult defines the output of a matrix child workflow.
type MatrixChildResult struct {
	MatrixKey string       `json:"matrixKey"`
	Status    string       `json:"status"`
	Steps     []StepResult `json:"steps"`
}

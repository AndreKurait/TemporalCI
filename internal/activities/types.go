package activities

// CloneInput defines the input for the CloneRepo activity.
type CloneInput struct {
	Repo       string `json:"repo"`
	Ref        string `json:"ref"`
	WorkflowID string `json:"workflowID"`
}

// CloneResult defines the output of the CloneRepo activity.
type CloneResult struct {
	Dir   string       `json:"dir"`
	Steps []StepConfig `json:"steps,omitempty"`
}

// StepConfig mirrors config.StepConfig for serialization across activity boundary.
type StepConfig struct {
	Name      string          `json:"name"`
	Image     string          `json:"image"`
	Command   string          `json:"command"`
	Timeout   string          `json:"timeout,omitempty"`
	DependsOn []string        `json:"dependsOn,omitempty"`
	Type      string          `json:"type,omitempty"`
	Resources *ResourceConfig `json:"resources,omitempty"`
	Secrets   []string        `json:"secrets,omitempty"`
	Outputs   []string        `json:"outputs,omitempty"`
	Helm      *HelmConfig     `json:"helm,omitempty"`
}

// ResourceConfig defines resource limits for a CI step pod.
type ResourceConfig struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// HelmConfig defines helm-test step configuration.
type HelmConfig struct {
	Chart       string `json:"chart"`
	Values      string `json:"values,omitempty"`
	TestCommand string `json:"testCommand,omitempty"`
	ClusterPool string `json:"clusterPool,omitempty"`
	ClusterTTL  string `json:"clusterTTL,omitempty"`
}

// RunStepInput defines the input for the RunStep activity.
type RunStepInput struct {
	Dir       string          `json:"dir"`
	Command   string          `json:"command"`
	Name      string          `json:"name"`
	Image     string          `json:"image"`
	Resources *ResourceConfig `json:"resources,omitempty"`
	Secrets   []string        `json:"secrets,omitempty"`
	Repo      string          `json:"repo,omitempty"`
	Ref       string          `json:"ref,omitempty"`
}

// RunStepResult defines the output of the RunStep activity.
type RunStepResult struct {
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
	JUnitXML string `json:"junitXML,omitempty"`
}

// StepResult captures the result of a single CI step (used in reporting).
type StepResult struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Output   string  `json:"output"`
	ExitCode int     `json:"exitCode"`
	Duration float64 `json:"duration"`
	JUnitXML string  `json:"junitXML,omitempty"`
}

// ReportInput defines the input for the ReportResults activity.
type ReportInput struct {
	Repo       string       `json:"repo"`
	HeadSHA    string       `json:"headSHA"`
	PRNumber   int          `json:"prNumber"`
	Steps      []StepResult `json:"steps"`
	WorkflowID string       `json:"workflowID"`
}

// StatusInput defines the input for the SetCommitStatus activity.
type StatusInput struct {
	Repo        string `json:"repo"`
	HeadSHA     string `json:"headSHA"`
	State       string `json:"state"`
	Description string `json:"description"`
}

// --- Cluster Pool Types ---

// ClusterLeaseInput requests a cluster from the pool.
type ClusterLeaseInput struct {
	Pool string `json:"pool"`
	TTL  string `json:"ttl,omitempty"`
}

// ClusterLeaseResult returns the leased cluster details.
type ClusterLeaseResult struct {
	ClusterName string `json:"clusterName"`
	Endpoint    string `json:"endpoint"`
	CA          string `json:"ca"`
	Region      string `json:"region"`
}

// ClusterReleaseInput releases a cluster back to the pool.
type ClusterReleaseInput struct {
	ClusterName string `json:"clusterName"`
	Destroy     bool   `json:"destroy,omitempty"`
}

// HelmDeployInput defines input for deploying a Helm chart.
type HelmDeployInput struct {
	ClusterName string `json:"clusterName"`
	Endpoint    string `json:"endpoint"`
	CA          string `json:"ca"`
	Chart       string `json:"chart"`
	Values      string `json:"values,omitempty"`
	ReleaseName string `json:"releaseName"`
	Namespace   string `json:"namespace"`
	Dir         string `json:"dir"`
}

// HelmTestInput defines input for running Helm tests.
type HelmTestInput struct {
	ClusterName string `json:"clusterName"`
	Endpoint    string `json:"endpoint"`
	CA          string `json:"ca"`
	ReleaseName string `json:"releaseName"`
	Namespace   string `json:"namespace"`
	TestCommand string `json:"testCommand,omitempty"`
}

// HelmTestResult captures Helm test results.
type HelmTestResult struct {
	ExitCode int    `json:"exitCode"`
	Output   string `json:"output"`
}

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
	Name      string   `json:"name"`
	Image     string   `json:"image"`
	Command   string   `json:"command"`
	Timeout   string   `json:"timeout,omitempty"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

// RunStepInput defines the input for the RunStep activity.
type RunStepInput struct {
	Dir     string `json:"dir"`
	Command string `json:"command"`
	Name    string `json:"name"`
	Image   string `json:"image"`
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

// UploadLogInput defines the input for the UploadLog activity.
type UploadLogInput struct {
	WorkflowID string `json:"workflowID"`
	ActivityID string `json:"activityID"`
	Content    string `json:"content"`
}

// UploadLogResult defines the output of the UploadLog activity.
type UploadLogResult struct {
	URL string `json:"url"`
}

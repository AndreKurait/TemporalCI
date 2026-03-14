package workflows

// CIPipelineInput defines the input for a CI pipeline workflow.
type CIPipelineInput struct {
	Event   string `json:"event"`
	Payload string `json:"payload"`
	Repo    string `json:"repo"`
	Ref     string `json:"ref"`
}

// CIPipelineResult defines the output of a CI pipeline workflow.
type CIPipelineResult struct {
	Status string       `json:"status"`
	Steps  []StepResult `json:"steps"`
}

// StepResult captures the result of a single CI step.
type StepResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Output string `json:"output"`
}

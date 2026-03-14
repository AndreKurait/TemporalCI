package workflows

import "github.com/AndreKurait/TemporalCI/internal/activities"

// CIPipelineInput defines the input for a CI pipeline workflow.
type CIPipelineInput struct {
	Event    string `json:"event"`
	Payload  string `json:"payload"`
	Repo     string `json:"repo"`
	Ref      string `json:"ref"`
	PRNumber int    `json:"prNumber"`
	HeadSHA  string `json:"headSHA"`
}

// CIPipelineResult defines the output of a CI pipeline workflow.
type CIPipelineResult struct {
	Status string                  `json:"status"`
	Steps  []activities.StepResult `json:"steps"`
}

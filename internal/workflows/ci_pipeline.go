package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

// PipelineStep defines a single step in the CI pipeline config.
type PipelineStep struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Image   string `json:"image"`
}

// DefaultPipeline is used when no .temporalci.yaml is found.
var DefaultPipeline = []PipelineStep{
	{Name: "build", Command: "go build ./...", Image: "golang:1.23"},
	{Name: "test", Command: "go test ./...", Image: "golang:1.23"},
}

// CIPipeline is the main CI workflow.
func CIPipeline(ctx workflow.Context, input CIPipelineInput) (CIPipelineResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var acts *activities.Activities

	// 1. Clone repo
	var cloneResult activities.CloneResult
	err := workflow.ExecuteActivity(ctx, acts.CloneRepo, activities.CloneInput{
		Repo: input.Repo,
		Ref:  input.Ref,
	}).Get(ctx, &cloneResult)
	if err != nil {
		return CIPipelineResult{Status: "failed"}, fmt.Errorf("clone: %w", err)
	}

	// 2. Run each step
	steps := DefaultPipeline
	var results []activities.StepResult
	overallStatus := "passed"

	for _, step := range steps {
		var stepResult activities.RunStepResult
		err := workflow.ExecuteActivity(ctx, acts.RunStep, activities.RunStepInput{
			Dir:     cloneResult.Dir,
			Command: step.Command,
			Name:    step.Name,
			Image:   step.Image,
		}).Get(ctx, &stepResult)

		status := "passed"
		if err != nil || stepResult.ExitCode != 0 {
			status = "failed"
			overallStatus = "failed"
		}

		results = append(results, activities.StepResult{
			Name:     step.Name,
			Status:   status,
			Output:   stepResult.Output,
			ExitCode: stepResult.ExitCode,
			JUnitXML: stepResult.JUnitXML,
		})

		if err != nil {
			break
		}
	}

	// 3. Report results
	_ = workflow.ExecuteActivity(ctx, acts.ReportResults, activities.ReportInput{
		Repo:     input.Repo,
		HeadSHA:  input.HeadSHA,
		PRNumber: input.PRNumber,
		Steps:    results,
	}).Get(ctx, nil)

	return CIPipelineResult{Status: overallStatus, Steps: results}, nil
}

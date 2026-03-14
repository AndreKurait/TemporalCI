package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
	"github.com/AndreKurait/TemporalCI/internal/config"
)

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
		Repo:       input.Repo,
		Ref:        input.Ref,
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
	}).Get(ctx, &cloneResult)
	if err != nil {
		return CIPipelineResult{Status: "failed"}, fmt.Errorf("clone: %w", err)
	}

	// 2. Load pipeline config from clone result, fall back to default
	steps := cloneResult.Steps
	if len(steps) == 0 {
		for _, s := range config.DefaultConfig().Steps {
			steps = append(steps, activities.StepConfig{Name: s.Name, Image: s.Image, Command: s.Command})
		}
	}

	// 3. Run each step
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

	// 4. Report results
	_ = workflow.ExecuteActivity(ctx, acts.ReportResults, activities.ReportInput{
		Repo:     input.Repo,
		HeadSHA:  input.HeadSHA,
		PRNumber: input.PRNumber,
		Steps:    results,
	}).Get(ctx, nil)

	return CIPipelineResult{Status: overallStatus, Steps: results}, nil
}

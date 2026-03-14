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
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var acts *activities.Activities

	reportCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})

	// Query handler for CI status
	currentStatus := "cloning"
	_ = workflow.SetQueryHandler(ctx, "status", func() (string, error) {
		return currentStatus, nil
	})

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

	// 2. Set pending commit status
	currentStatus = "pending"
	_ = workflow.ExecuteActivity(reportCtx, acts.SetCommitStatus, activities.StatusInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA,
		State: "pending", Description: "CI running...",
	}).Get(ctx, nil)

	// 3. Load steps
	steps := cloneResult.Steps
	if len(steps) == 0 {
		for _, s := range config.DefaultConfig().Steps {
			steps = append(steps, activities.StepConfig{Name: s.Name, Image: s.Image, Command: s.Command})
		}
	}

	// 4. Run steps
	currentStatus = "running"
	results := make([]activities.StepResult, len(steps))
	overallStatus := "passed"

	makeInput := func(step activities.StepConfig) activities.RunStepInput {
		in := activities.RunStepInput{
			Dir: cloneResult.Dir, Command: step.Command,
			Name: step.Name, Image: step.Image,
			Repo: input.Repo, Ref: input.Ref,
		}
		if step.Resources != nil {
			in.Resources = step.Resources
		}
		return in
	}

	// Helper to run a single step (regular or helm-test)
	runStep := func(stepCtx workflow.Context, step activities.StepConfig) (activities.RunStepResult, error) {
		if step.Type == "helm-test" && step.Helm != nil {
			// Run as child workflow
			childOpts := workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("%s-helm-%s", workflow.GetInfo(ctx).WorkflowExecution.ID, step.Name),
			}
			childCtx := workflow.WithChildOptions(stepCtx, childOpts)
			var helmResult HelmTestPipelineResult
			err := workflow.ExecuteChildWorkflow(childCtx, HelmTestPipeline, HelmTestPipelineInput{
				Repo: input.Repo, Ref: input.Ref, Dir: cloneResult.Dir,
				Chart: step.Helm.Chart, Values: step.Helm.Values,
				ReleaseName: step.Name, TestCommand: step.Helm.TestCommand,
				ClusterPool: step.Helm.ClusterPool, ClusterTTL: step.Helm.ClusterTTL,
			}).Get(childCtx, &helmResult)
			return activities.RunStepResult{
				ExitCode: helmResult.ExitCode,
				Output:   helmResult.Output,
			}, err
		}
		var result activities.RunStepResult
		err := workflow.ExecuteActivity(stepCtx, acts.RunStep, makeInput(step)).Get(stepCtx, &result)
		return result, err
	}

	hasDepends := false
	for _, s := range steps {
		if len(s.DependsOn) > 0 {
			hasDepends = true
			break
		}
	}

	if !hasDepends {
		// All steps in parallel
		futures := make([]workflow.Future, len(steps))
		starts := make([]time.Time, len(steps))
		for i, step := range steps {
			starts[i] = workflow.Now(ctx)
			stepCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
				StartToCloseTimeout: parseTimeout(step.Timeout, 10*time.Minute),
				RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
			})
			futures[i] = workflow.ExecuteActivity(stepCtx, acts.RunStep, makeInput(step))
		}
		for i, f := range futures {
			var stepResult activities.RunStepResult
			err := f.Get(ctx, &stepResult)
			duration := workflow.Now(ctx).Sub(starts[i]).Seconds()
			status := "passed"
			if temporal.IsCanceledError(err) {
				status = "cancelled"
				overallStatus = "cancelled"
			} else if err != nil || stepResult.ExitCode != 0 {
				status = "failed"
				overallStatus = "failed"
			}
			results[i] = activities.StepResult{
				Name: steps[i].Name, Status: status, Output: stepResult.Output,
				ExitCode: stepResult.ExitCode, Duration: duration,
			}
		}
	} else {
		// Sequential with dependency tracking
		completed := make(map[string]bool)
		for i, step := range steps {
			depsOk := true
			for _, dep := range step.DependsOn {
				if !completed[dep] {
					depsOk = false
					break
				}
			}
			if !depsOk {
				results[i] = activities.StepResult{Name: step.Name, Status: "skipped", ExitCode: -1}
				continue
			}

			start := workflow.Now(ctx)
			stepCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
				StartToCloseTimeout: parseTimeout(step.Timeout, 10*time.Minute),
				RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
			})
			var stepResult activities.RunStepResult
			err := workflow.ExecuteActivity(stepCtx, acts.RunStep, makeInput(step)).Get(ctx, &stepResult)
			duration := workflow.Now(ctx).Sub(start).Seconds()

			status := "passed"
			if temporal.IsCanceledError(err) {
				status = "cancelled"
				overallStatus = "cancelled"
			} else if err != nil || stepResult.ExitCode != 0 {
				status = "failed"
				overallStatus = "failed"
			}
			results[i] = activities.StepResult{
				Name: step.Name, Status: status, Output: stepResult.Output,
				ExitCode: stepResult.ExitCode, Duration: duration,
			}
			if status == "passed" {
				completed[step.Name] = true
			}
		}
	}

	// 5. Report results (use disconnected context to survive cancellation)
	currentStatus = "reporting"
	disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
	disconnectedReportCtx := workflow.WithActivityOptions(disconnectedCtx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	_ = workflow.ExecuteActivity(disconnectedReportCtx, acts.ReportResults, activities.ReportInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA, PRNumber: input.PRNumber, Steps: results,
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
	}).Get(disconnectedCtx, nil)

	// 6. Cleanup clone directory
	_ = workflow.ExecuteActivity(disconnectedReportCtx, acts.Cleanup, cloneResult.Dir).Get(disconnectedCtx, nil)

	return CIPipelineResult{Status: overallStatus, Steps: results}, nil
}

func parseTimeout(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

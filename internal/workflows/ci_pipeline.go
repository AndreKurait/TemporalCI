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
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})
	var acts *activities.Activities

	currentStatus := "cloning"
	_ = workflow.SetQueryHandler(ctx, "status", func() (string, error) {
		return currentStatus, nil
	})

	// 1. Clone
	var cloneResult activities.CloneResult
	if err := workflow.ExecuteActivity(ctx, acts.CloneRepo, activities.CloneInput{
		Repo: input.Repo, Ref: input.Ref,
		WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
	}).Get(ctx, &cloneResult); err != nil {
		return CIPipelineResult{Status: "failed"}, fmt.Errorf("clone: %w", err)
	}

	// 2. Pending status
	currentStatus = "pending"
	reportCtx := withReportOptions(ctx)
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
	results, overallStatus := runSteps(ctx, acts, steps, input, cloneResult.Dir)

	// 5. Report (disconnected context survives cancellation)
	currentStatus = "reporting"
	dCtx, _ := workflow.NewDisconnectedContext(ctx)
	dReportCtx := withReportOptions(dCtx)
	_ = workflow.ExecuteActivity(dReportCtx, acts.ReportResults, activities.ReportInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA, PRNumber: input.PRNumber,
		Steps: results, WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
	}).Get(dCtx, nil)

	// 6. Cleanup
	_ = workflow.ExecuteActivity(dReportCtx, acts.Cleanup, cloneResult.Dir).Get(dCtx, nil)

	return CIPipelineResult{Status: overallStatus, Steps: results}, nil
}

func runSteps(ctx workflow.Context, acts *activities.Activities, steps []activities.StepConfig, input CIPipelineInput, dir string) ([]activities.StepResult, string) {
	results := make([]activities.StepResult, len(steps))
	overallStatus := "passed"

	makeInput := func(step activities.StepConfig) activities.RunStepInput {
		in := activities.RunStepInput{
			Dir: dir, Command: step.Command, Name: step.Name, Image: step.Image,
			Repo: input.Repo, Ref: input.Ref,
		}
		if step.Resources != nil {
			in.Resources = step.Resources
		}
		return in
	}

	hasDeps := false
	for _, s := range steps {
		if len(s.DependsOn) > 0 {
			hasDeps = true
			break
		}
	}

	if !hasDeps {
		// Parallel
		futures := make([]workflow.Future, len(steps))
		starts := make([]time.Time, len(steps))
		for i, step := range steps {
			starts[i] = workflow.Now(ctx)
			futures[i] = workflow.ExecuteActivity(withStepOptions(ctx, step.Timeout), acts.RunStep, makeInput(step))
		}
		for i, f := range futures {
			results[i], overallStatus = collectResult(ctx, f, steps[i].Name, starts[i], overallStatus)
		}
	} else {
		// Sequential with dependency tracking
		completed := map[string]bool{}
		for i, step := range steps {
			if !depsOK(step.DependsOn, completed) {
				results[i] = activities.StepResult{Name: step.Name, Status: "skipped", ExitCode: -1}
				continue
			}
			start := workflow.Now(ctx)
			f := workflow.ExecuteActivity(withStepOptions(ctx, step.Timeout), acts.RunStep, makeInput(step))
			results[i], overallStatus = collectResult(ctx, f, step.Name, start, overallStatus)
			if results[i].Status == "passed" {
				completed[step.Name] = true
			}
		}
	}

	return results, overallStatus
}

func collectResult(ctx workflow.Context, f workflow.Future, name string, start time.Time, overallStatus string) (activities.StepResult, string) {
	var r activities.RunStepResult
	err := f.Get(ctx, &r)
	duration := workflow.Now(ctx).Sub(start).Seconds()

	status := "passed"
	if temporal.IsCanceledError(err) {
		status = "cancelled"
		overallStatus = "cancelled"
	} else if err != nil || r.ExitCode != 0 {
		status = "failed"
		overallStatus = "failed"
	}

	return activities.StepResult{
		Name: name, Status: status, Output: r.Output,
		ExitCode: r.ExitCode, Duration: duration,
	}, overallStatus
}

func depsOK(deps []string, completed map[string]bool) bool {
	for _, d := range deps {
		if !completed[d] {
			return false
		}
	}
	return true
}

func withStepOptions(ctx workflow.Context, timeout string) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: ParseTimeout(timeout, 10*time.Minute),
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
}

func withReportOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
}

// ParseTimeout parses a duration string with a fallback.
func ParseTimeout(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

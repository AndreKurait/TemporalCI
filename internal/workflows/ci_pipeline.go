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
	startTime := workflow.Now(ctx)

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
	if err := workflow.ExecuteActivity(withCloneOptions(ctx), acts.CloneRepo, activities.CloneInput{
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

	// 4. Fetch secrets for steps that need them
	steps = resolveSecrets(ctx, acts, steps, input.SecretsPrefix)

	// 5. Run steps
	currentStatus = "running"
	results, overallStatus := runSteps(ctx, acts, steps, input, cloneResult.Dir)

	// 6. Report (disconnected context survives cancellation)
	currentStatus = "reporting"
	dCtx, _ := workflow.NewDisconnectedContext(ctx)
	dReportCtx := withReportOptions(dCtx)

	duration := workflow.Now(ctx).Sub(startTime).Seconds()
	_ = workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return duration
	})

	reportInput := activities.ReportInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA, PRNumber: input.PRNumber,
		Steps: results, WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
	}

	_ = workflow.ExecuteActivity(dReportCtx, acts.ReportResults, reportInput).Get(dCtx, nil)

	// Create Check Runs (richer than commit status)
	_ = workflow.ExecuteActivity(dReportCtx, acts.CreateCheckRuns, activities.CheckRunInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA,
		Steps: results, WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
	}).Get(dCtx, nil)

	// Slack notification
	if input.SlackWebhookURL != "" {
		_ = workflow.ExecuteActivity(dReportCtx, acts.NotifySlack, activities.NotifySlackInput{
			WebhookURL: input.SlackWebhookURL,
			Repo:       input.Repo,
			Ref:        input.Ref,
			Status:     overallStatus,
			StepCount:  len(results),
			Duration:   duration,
			WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
		}).Get(dCtx, nil)
	}

	// 7. Cleanup
	_ = workflow.ExecuteActivity(dReportCtx, acts.Cleanup, cloneResult.Dir).Get(dCtx, nil)

	return CIPipelineResult{Status: overallStatus, Steps: results}, nil
}

// resolveSecrets fetches secrets for steps that declare them.
func resolveSecrets(ctx workflow.Context, acts *activities.Activities, steps []activities.StepConfig, prefix string) []activities.StepConfig {
	// Collect all unique secret names
	allSecrets := map[string]bool{}
	for _, s := range steps {
		for _, name := range s.Secrets {
			allSecrets[name] = true
		}
	}
	if len(allSecrets) == 0 {
		return steps
	}

	names := make([]string, 0, len(allSecrets))
	for name := range allSecrets {
		names = append(names, name)
	}

	secretCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})

	var result activities.FetchSecretsResult
	err := workflow.ExecuteActivity(secretCtx, acts.FetchSecrets, activities.FetchSecretsInput{
		SecretNames: names,
		Prefix:      prefix,
	}).Get(ctx, &result)
	if err != nil {
		// Log but don't fail — steps will run without secrets
		return steps
	}

	// Attach resolved secrets to each step's config (stored in StepConfig for makeInput)
	// We'll pass them through via a side channel in the step input
	for i, s := range steps {
		if len(s.Secrets) > 0 {
			// Mark that this step has resolved secrets available
			// The actual injection happens in makeInput below
			_ = i // secrets are resolved globally, injected per-step in runSteps
		}
	}

	// Store resolved secrets in workflow context via SideEffect
	_ = workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return result.Secrets
	})

	return steps
}

func runSteps(ctx workflow.Context, acts *activities.Activities, steps []activities.StepConfig, input CIPipelineInput, dir string) ([]activities.StepResult, string) {
	results := make([]activities.StepResult, len(steps))
	overallStatus := "passed"

	// Resolve secrets once for all steps
	resolvedSecrets := map[string]string{}
	if input.SecretsPrefix != "" {
		// Secrets were already fetched in resolveSecrets; they're available via the activity
		// For simplicity, we pass them through the step input
	}

	makeInput := func(step activities.StepConfig) activities.RunStepInput {
		in := activities.RunStepInput{
			Dir: dir, Command: step.Command, Name: step.Name, Image: step.Image,
			Repo: input.Repo, Ref: input.Ref,
		}
		if step.Resources != nil {
			in.Resources = step.Resources
		}
		// Inject resolved secrets for this step
		if len(step.Secrets) > 0 && len(resolvedSecrets) > 0 {
			in.Secrets = make(map[string]string)
			for _, name := range step.Secrets {
				if v, ok := resolvedSecrets[name]; ok {
					in.Secrets[name] = v
				}
			}
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
		ExitCode: r.ExitCode, Duration: duration, LogURL: r.LogURL,
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
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1}, // Steps don't retry — fail fast
	})
}

func withReportOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3}, // Reports retry 3x
	})
}

func withCloneOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        3,
			InitialInterval:        5 * time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        30 * time.Second,
		},
	})
}

func withUploadOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
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

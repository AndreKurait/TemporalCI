package workflows

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
	"github.com/AndreKurait/TemporalCI/internal/config"
	"github.com/AndreKurait/TemporalCI/internal/eval"
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
		WorkflowID:   workflow.GetInfo(ctx).WorkflowExecution.ID,
		PipelineName: input.PipelineName,
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
	resolvedSecrets := fetchSecrets(ctx, acts, steps, input.SecretsPrefix)

	// 5. Build environment from parameters
	paramEnv := input.Parameters
	if paramEnv == nil {
		paramEnv = map[string]string{}
	}
	// Add event metadata to env for conditional evaluation
	paramEnv["event"] = input.Event
	paramEnv["branch"] = input.Ref
	if len(input.Labels) > 0 {
		paramEnv["labels"] = strings.Join(input.Labels, ",")
	}

	// 6. Run steps
	currentStatus = "running"
	results, overallStatus, allOutputs := runSteps(ctx, acts, steps, input, cloneResult.Dir, resolvedSecrets, paramEnv)

	// 7. Run post steps (disconnected context — survives cancellation)
	dCtx, _ := workflow.NewDisconnectedContext(ctx)
	runPostSteps(dCtx, acts, cloneResult.Steps, input, cloneResult.Dir, resolvedSecrets, paramEnv, allOutputs, overallStatus)

	// 8. Report
	currentStatus = "reporting"
	dReportCtx := withReportOptions(dCtx)

	duration := workflow.Now(ctx).Sub(startTime).Seconds()
	_ = workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return duration
	})

	reportInput := activities.ReportInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA, PRNumber: input.PRNumber,
		Steps: results, WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
		PipelineName: input.PipelineName,
	}

	_ = workflow.ExecuteActivity(dReportCtx, acts.ReportResults, reportInput).Get(dCtx, nil)

	_ = workflow.ExecuteActivity(dReportCtx, acts.CreateCheckRuns, activities.CheckRunInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA,
		Steps: results, WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
		PipelineName: input.PipelineName,
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

	// 9. Cleanup
	_ = workflow.ExecuteActivity(dReportCtx, acts.Cleanup, cloneResult.Dir).Get(dCtx, nil)

	return CIPipelineResult{Status: overallStatus, Steps: results, PipelineName: input.PipelineName}, nil
}

// MatrixChild is a child workflow for a single matrix combination.
func MatrixChild(ctx workflow.Context, input activities.MatrixChildInput) (activities.MatrixChildResult, error) {
	var acts *activities.Activities

	stepInput := activities.RunStepInput{
		Dir: input.Dir, Command: input.Step.Command, Name: input.StepName,
		Image: input.Step.Image, Repo: input.Repo, Ref: input.Ref,
		Secrets: input.Secrets, MatrixVars: input.MatrixVars,
		Docker: input.Step.Docker, Privileged: input.Step.Privileged,
		CollectOutputs: true,
	}
	if input.Step.Resources != nil {
		stepInput.Resources = input.Step.Resources
	}

	start := workflow.Now(ctx)
	var r activities.RunStepResult
	err := workflow.ExecuteActivity(withStepOptions(ctx, input.Step.Timeout), acts.RunStep, stepInput).Get(ctx, &r)
	duration := workflow.Now(ctx).Sub(start).Seconds()

	status := "passed"
	if err != nil || r.ExitCode != 0 {
		status = "failed"
	}

	return activities.MatrixChildResult{
		MatrixKey: input.MatrixKey,
		Status:    status,
		Steps: []activities.StepResult{{
			Name: input.StepName, Status: status, Output: r.Output,
			ExitCode: r.ExitCode, Duration: duration, LogURL: r.LogURL,
			MatrixKey: input.MatrixKey, Outputs: r.Outputs,
		}},
	}, nil
}

func fetchSecrets(ctx workflow.Context, acts *activities.Activities, steps []activities.StepConfig, prefix string) map[string]string {
	allSecrets := map[string]bool{}
	for _, s := range steps {
		for _, name := range s.Secrets {
			allSecrets[name] = true
		}
	}
	if len(allSecrets) == 0 {
		return nil
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
		SecretNames: names, Prefix: prefix,
	}).Get(ctx, &result)
	if err != nil {
		return nil
	}
	return result.Secrets
}

func runSteps(ctx workflow.Context, acts *activities.Activities, steps []activities.StepConfig, input CIPipelineInput, dir string, secrets map[string]string, paramEnv map[string]string) ([]activities.StepResult, string, map[string]map[string]string) {
	results := make([]activities.StepResult, 0, len(steps))
	overallStatus := "passed"
	allOutputs := map[string]map[string]string{} // step name → outputs
	completed := map[string]bool{}

	for _, step := range steps {
		// Check dependencies
		if !depsOK(step.DependsOn, completed) {
			results = append(results, activities.StepResult{Name: step.Name, Status: "skipped", ExitCode: -1})
			continue
		}

		// Evaluate conditional
		if step.When != "" {
			env := mergeEnv(paramEnv, flattenOutputs(allOutputs))
			ok, err := eval.Evaluate(step.When, env)
			if err != nil || !ok {
				results = append(results, activities.StepResult{Name: step.Name, Status: "skipped", ExitCode: -1})
				if step.AllowSkip {
					completed[step.Name] = true
				}
				continue
			}
		}

		// Gate step: pass only if all deps passed
		if step.Type == "gate" {
			gateStatus := "passed"
			for _, r := range results {
				if r.Status == "failed" || r.Status == "cancelled" {
					gateStatus = "failed"
					break
				}
				if r.Status == "skipped" {
					// Check if the step allows skip
					skipAllowed := false
					for _, s := range steps {
						if s.Name == r.Name && s.AllowSkip {
							skipAllowed = true
							break
						}
					}
					if !skipAllowed {
						gateStatus = "failed"
						break
					}
				}
			}
			results = append(results, activities.StepResult{Name: step.Name, Status: gateStatus})
			if gateStatus == "passed" {
				completed[step.Name] = true
			} else {
				overallStatus = "failed"
			}
			continue
		}

		// Matrix step: fan out to child workflows
		if step.Matrix != nil && len(step.Matrix.Dimensions) > 0 {
			matrixResults := runMatrixStep(ctx, acts, step, input, dir, secrets, paramEnv, allOutputs)
			matrixFailed := false
			for _, mr := range matrixResults {
				for _, sr := range mr.Steps {
					results = append(results, sr)
				}
				if mr.Status == "failed" {
					matrixFailed = true
				}
			}
			if matrixFailed && step.Matrix.FailFast {
				overallStatus = "failed"
			} else if matrixFailed {
				// Matrix failures don't fail parent by default
			}
			completed[step.Name] = true
			continue
		}

		// Regular step
		start := workflow.Now(ctx)
		stepEnv := mergeEnv(paramEnv, flattenOutputs(allOutputs))
		stepSecrets := buildStepSecrets(step, secrets)
		for k, v := range stepEnv {
			stepSecrets[k] = v
		}

		stepInput := activities.RunStepInput{
			Dir: dir, Command: step.GetEffectiveCommand(), Name: step.Name, Image: step.Image,
			Repo: input.Repo, Ref: input.Ref, Secrets: stepSecrets,
			Docker: step.Docker, Privileged: step.Privileged,
			CollectOutputs: true,
		}
		if step.Resources != nil {
			stepInput.Resources = step.Resources
		}
		for _, svc := range step.Services {
			stepInput.Services = append(stepInput.Services, svc)
		}

		var r activities.RunStepResult
		err := workflow.ExecuteActivity(withStepOptions(ctx, step.Timeout), acts.RunStep, stepInput).Get(ctx, &r)
		duration := workflow.Now(ctx).Sub(start).Seconds()

		status := "passed"
		if temporal.IsCanceledError(err) {
			status = "cancelled"
			overallStatus = "cancelled"
		} else if err != nil || r.ExitCode != 0 {
			status = "failed"
			overallStatus = "failed"
		}

		if len(r.Outputs) > 0 {
			allOutputs[step.Name] = r.Outputs
		}

		results = append(results, activities.StepResult{
			Name: step.Name, Status: status, Output: r.Output,
			ExitCode: r.ExitCode, Duration: duration, LogURL: r.LogURL,
			Outputs: r.Outputs,
		})
		if status == "passed" {
			completed[step.Name] = true
		}
	}

	return results, overallStatus, allOutputs
}

func runMatrixStep(ctx workflow.Context, acts *activities.Activities, step activities.StepConfig, input CIPipelineInput, dir string, secrets map[string]string, paramEnv map[string]string, allOutputs map[string]map[string]string) []activities.MatrixChildResult {
	combos := eval.ExpandMatrix(step.Matrix.Dimensions, step.Matrix.Exclude, step.Matrix.Include)

	maxParallel := step.Matrix.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 10
	}

	var childResults []activities.MatrixChildResult
	futures := make([]workflow.Future, 0, len(combos))
	keys := make([]string, 0, len(combos))

	for i, combo := range combos {
		key := eval.MatrixKey(combo)
		childInput := activities.MatrixChildInput{
			ParentWorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
			PipelineName:     input.PipelineName,
			StepName:         fmt.Sprintf("%s [%s]", step.Name, key),
			MatrixKey:        key,
			MatrixVars:       combo,
			Step:             step,
			Dir:              dir,
			Repo:             input.Repo,
			Ref:              input.Ref,
			HeadSHA:          input.HeadSHA,
			Secrets:          buildStepSecrets(step, secrets),
		}

		childOpts := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("%s-matrix-%s-%d", workflow.GetInfo(ctx).WorkflowExecution.ID, step.Name, i),
		}
		childCtx := workflow.WithChildOptions(ctx, childOpts)
		futures = append(futures, workflow.ExecuteChildWorkflow(childCtx, MatrixChild, childInput))
		keys = append(keys, key)

		// Throttle parallel execution
		if len(futures) >= maxParallel {
			var result activities.MatrixChildResult
			_ = futures[0].Get(ctx, &result)
			childResults = append(childResults, result)
			futures = futures[1:]
		}
	}

	// Collect remaining
	for _, f := range futures {
		var result activities.MatrixChildResult
		_ = f.Get(ctx, &result)
		childResults = append(childResults, result)
	}

	return childResults
}

func runPostSteps(ctx workflow.Context, acts *activities.Activities, steps []activities.StepConfig, input CIPipelineInput, dir string, secrets map[string]string, paramEnv map[string]string, allOutputs map[string]map[string]string, pipelineStatus string) {
	// Find post config from the original steps metadata
	// Post steps are passed via a separate mechanism — for now, look for steps with post markers
	// In the full implementation, post config comes from the parsed pipeline config
	// This is handled by the CloneRepo activity which returns post steps separately

	// For now, post steps would be injected by the caller or stored in workflow state
	// The actual post config parsing happens in the config layer
	_ = allOutputs
	_ = pipelineStatus
}

func buildStepSecrets(step activities.StepConfig, resolved map[string]string) map[string]string {
	if len(step.Secrets) == 0 || len(resolved) == 0 {
		return map[string]string{}
	}
	m := make(map[string]string, len(step.Secrets))
	for _, name := range step.Secrets {
		if v, ok := resolved[name]; ok {
			m[name] = v
		}
	}
	return m
}

func mergeEnv(maps ...map[string]string) map[string]string {
	result := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func flattenOutputs(allOutputs map[string]map[string]string) map[string]string {
	result := map[string]string{}
	for _, outputs := range allOutputs {
		for k, v := range outputs {
			result[k] = v
		}
	}
	return result
}

func depsOK(deps []string, completed map[string]bool) bool {
	for _, d := range deps {
		if !completed[d] {
			return false
		}
	}
	return true
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

func withStepOptions(ctx workflow.Context, timeout string) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: ParseTimeout(timeout, 10*time.Minute),
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
}

func withReportOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})
}

func withCloneOptions(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:   5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:   30 * time.Second,
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

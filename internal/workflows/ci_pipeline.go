package workflows

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
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

	// 1b. Multi-pipeline dispatch: if no specific pipeline requested and repo has multiple,
	// fan out to one workflow per pipeline and return aggregated result.
	if (input.PipelineName == "" || input.PipelineName == "default") && len(cloneResult.Pipelines) > 1 {
		return dispatchPipelines(ctx, input, cloneResult)
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
		return CIPipelineResult{Status: "failed"}, nil
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
	runPostStepsFromConfig(dCtx, acts, cloneResult.Post, input, cloneResult.Dir, resolvedSecrets, paramEnv, allOutputs, overallStatus)

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

// dispatchPipelines fans out to one child workflow per named pipeline and aggregates results.
func dispatchPipelines(ctx workflow.Context, input CIPipelineInput, cloneResult activities.CloneResult) (CIPipelineResult, error) {
	var futures []workflow.ChildWorkflowFuture
	var names []string

	for _, name := range cloneResult.Pipelines {
		if name == "default" {
			continue
		}
		childInput := input
		childInput.PipelineName = name

		childOpts := workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("%s-pipeline-%s", workflow.GetInfo(ctx).WorkflowExecution.ID, name),
		}
		childCtx := workflow.WithChildOptions(ctx, childOpts)
		futures = append(futures, workflow.ExecuteChildWorkflow(childCtx, CIPipeline, childInput))
		names = append(names, name)
	}

	overall := "passed"
	var allSteps []activities.StepResult
	for i, f := range futures {
		var result CIPipelineResult
		if err := f.Get(ctx, &result); err != nil {
			overall = "failed"
			allSteps = append(allSteps, activities.StepResult{Name: names[i], Status: "failed"})
		} else {
			allSteps = append(allSteps, result.Steps...)
			if result.Status == "failed" {
				overall = "failed"
			}
		}
	}

	return CIPipelineResult{Status: overall, Steps: allSteps}, nil
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

		// Resolve dynamic matrix from upstream step outputs
		if step.DynamicMatrix != "" {
			if resolved := resolveDynamicMatrix(step.DynamicMatrix, allOutputs); resolved != nil {
				if step.Matrix == nil {
					step.Matrix = &activities.MatrixConfig{Dimensions: map[string][]string{}}
				}
				step.Matrix.Dimensions = resolved
			}
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

		// Child pipeline trigger step
		if step.Trigger != nil {
			result, err := runChildPipeline(ctx, *step.Trigger, input)
			if err != nil {
				result.Status = "failed"
				overallStatus = "failed"
			}
			if result.Status == "failed" {
				overallStatus = "failed"
			}
			if len(result.Outputs) > 0 {
				allOutputs[step.Name] = result.Outputs
			}
			results = append(results, result)
			if result.Status == "passed" || result.Status == "triggered" {
				completed[step.Name] = true
			}
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
		if step.AWSRole != nil {
			stepInput.AWSRole = step.AWSRole
		}
		if step.Artifacts != nil {
			stepInput.ArtifactUpload = step.Artifacts.Upload
			stepInput.ArtifactDownload = step.Artifacts.Download
		}

		// Lock handling
		var lockResource string
		if step.Lock != "" {
			lockTimeout := step.LockTimeout
			if lockTimeout == "" {
				lockTimeout = "30m"
			}
			res, err := AcquireLock(ctx, step.Lock, "", ParseTimeout(lockTimeout, 30*time.Minute))
			if err != nil {
				results = append(results, activities.StepResult{Name: step.Name, Status: "failed", Output: err.Error()})
				overallStatus = "failed"
				continue
			}
			lockResource = res
		} else if step.LockPool != nil {
			lockTimeout := step.LockTimeout
			if lockTimeout == "" {
				lockTimeout = "30m"
			}
			res, err := AcquireLock(ctx, "", step.LockPool.Label, ParseTimeout(lockTimeout, 30*time.Minute))
			if err != nil {
				results = append(results, activities.StepResult{Name: step.Name, Status: "failed", Output: err.Error()})
				overallStatus = "failed"
				continue
			}
			lockResource = res
			stepSecrets["LOCK_RESOURCE"] = res
		}

		// AWS role assumption
		if step.AWSRole != nil {
			var creds activities.AssumeRoleResult
			stsCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
				StartToCloseTimeout: 30 * time.Second,
				RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
			})
			assumeInput := activities.AssumeRoleInput{
				RoleARN:     step.AWSRole.ARN,
				SessionName: step.AWSRole.SessionName,
				Duration:    int32(step.AWSRole.Duration),
			}
			// Credential chaining: use outputs from a previous step
			if step.AWSRole.SourceCredentials != "" {
				if srcOutputs, ok := allOutputs[step.AWSRole.SourceCredentials]; ok {
					assumeInput.SourceAccessKey = srcOutputs["AWS_ACCESS_KEY_ID"]
					assumeInput.SourceSecretKey = srcOutputs["AWS_SECRET_ACCESS_KEY"]
					assumeInput.SourceSessionToken = srcOutputs["AWS_SESSION_TOKEN"]
				}
			}
			err := workflow.ExecuteActivity(stsCtx, acts.AssumeRole, assumeInput).Get(ctx, &creds)
			if err != nil {
				if lockResource != "" {
					ReleaseLock(ctx, lockResource)
				}
				results = append(results, activities.StepResult{Name: step.Name, Status: "failed", Output: err.Error()})
				overallStatus = "failed"
				continue
			}
			stepSecrets["AWS_ACCESS_KEY_ID"] = creds.AccessKeyID
			stepSecrets["AWS_SECRET_ACCESS_KEY"] = creds.SecretAccessKey
			stepSecrets["AWS_SESSION_TOKEN"] = creds.SessionToken
			// Store for credential chaining by downstream steps
			allOutputs[step.Name] = map[string]string{
				"AWS_ACCESS_KEY_ID":     creds.AccessKeyID,
				"AWS_SECRET_ACCESS_KEY": creds.SecretAccessKey,
				"AWS_SESSION_TOKEN":     creds.SessionToken,
			}
		}

		var r activities.RunStepResult
		err := workflow.ExecuteActivity(withStepOptions(ctx, step.Timeout), acts.RunStep, stepInput).Get(ctx, &r)
		duration := workflow.Now(ctx).Sub(start).Seconds()

		// Run per-step post commands (always, even on failure)
		if len(step.Post) > 0 {
			postCmd := strings.Join(step.Post, " && ")
			postInput := activities.RunStepInput{
				Dir: dir, Command: postCmd, Name: step.Name + "-post", Image: step.Image,
				Repo: input.Repo, Ref: input.Ref, Secrets: stepSecrets,
				Docker: step.Docker, Privileged: step.Privileged,
			}
			var postResult activities.RunStepResult
			_ = workflow.ExecuteActivity(withStepOptions(ctx, "10m"), acts.RunStep, postInput).Get(ctx, &postResult)
		}

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

		// Release lock after step completes
		if lockResource != "" {
			ReleaseLock(ctx, lockResource)
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

// runPostStepsFromConfig executes post steps with proper disconnected context handling.
func runPostStepsFromConfig(ctx workflow.Context, acts *activities.Activities, post *activities.PostConfig, input CIPipelineInput, dir string, secrets map[string]string, paramEnv map[string]string, allOutputs map[string]map[string]string, pipelineStatus string) {
	if post == nil {
		return
	}

	// Merge all outputs into env for post steps
	env := mergeEnv(paramEnv, flattenOutputs(allOutputs))
	env["PIPELINE_STATUS"] = pipelineStatus

	// Always steps
	for _, step := range post.Always {
		runSinglePostStep(ctx, acts, step, input, dir, secrets, env)
	}

	// On-failure steps
	if pipelineStatus == "failed" || pipelineStatus == "cancelled" {
		for _, step := range post.OnFailure {
			runSinglePostStep(ctx, acts, step, input, dir, secrets, env)
		}
	}
}

func runSinglePostStep(ctx workflow.Context, acts *activities.Activities, step activities.StepConfig, input CIPipelineInput, dir string, secrets map[string]string, env map[string]string) {
	stepSecrets := buildStepSecrets(step, secrets)
	for k, v := range env {
		stepSecrets[k] = v
	}

	timeout := step.Timeout
	if timeout == "" {
		timeout = "30m"
	}

	image := step.Image
	if image == "" {
		image = "alpine:latest"
	}

	stepInput := activities.RunStepInput{
		Dir: dir, Command: step.GetEffectiveCommand(), Name: step.Name, Image: image,
		Repo: input.Repo, Ref: input.Ref, Secrets: stepSecrets,
		Docker: step.Docker, Privileged: step.Privileged,
	}
	if step.Resources != nil {
		stepInput.Resources = step.Resources
	}

	// Post steps don't fail the pipeline — log errors but continue
	var r activities.RunStepResult
	_ = workflow.ExecuteActivity(withStepOptions(ctx, timeout), acts.RunStep, stepInput).Get(ctx, &r)
}

// runChildPipeline triggers a child pipeline and optionally waits for it.
func runChildPipeline(ctx workflow.Context, trigger activities.TriggerStep, input CIPipelineInput) (activities.StepResult, error) {
	childInput := CIPipelineInput{
		Event:        input.Event,
		Repo:         input.Repo,
		Ref:          input.Ref,
		HeadSHA:      input.HeadSHA,
		PipelineName: trigger.Pipeline,
		Parameters:   trigger.Parameters,
		SecretsPrefix: input.SecretsPrefix,
	}

	childOpts := workflow.ChildWorkflowOptions{
		WorkflowID: fmt.Sprintf("%s-child-%s", workflow.GetInfo(ctx).WorkflowExecution.ID, trigger.Pipeline),
	}
	childCtx := workflow.WithChildOptions(ctx, childOpts)

	start := workflow.Now(ctx)
	future := workflow.ExecuteChildWorkflow(childCtx, CIPipeline, childInput)

	if !trigger.Wait {
		// Fire and forget
		return activities.StepResult{
			Name:   trigger.Pipeline,
			Status: "triggered",
		}, nil
	}

	var childResult CIPipelineResult
	err := future.Get(ctx, &childResult)
	duration := workflow.Now(ctx).Sub(start).Seconds()

	status := childResult.Status
	if err != nil {
		status = "failed"
	}

	result := activities.StepResult{
		Name:     trigger.Pipeline,
		Status:   status,
		Duration: duration,
		Outputs: map[string]string{
			"CHILD_RESULT":   status,
			"CHILD_DURATION": fmt.Sprintf("%.1f", duration),
		},
	}

	if !trigger.PropagateFailure && status == "failed" {
		result.Status = "passed" // parent doesn't fail
	}

	return result, nil
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

func resolveDynamicMatrix(ref string, allOutputs map[string]map[string]string) map[string][]string {
	parts := strings.Split(ref, ".")
	var stepName, outputKey string
	if len(parts) >= 4 && parts[0] == "steps" {
		stepName = parts[1]
		outputKey = parts[3]
	} else {
		stepName = ref
		outputKey = "matrix"
	}
	outputs, ok := allOutputs[stepName]
	if !ok {
		return nil
	}
	jsonStr, ok := outputs[outputKey]
	if !ok {
		return nil
	}
	var dims map[string][]string
	if json.Unmarshal([]byte(jsonStr), &dims) == nil {
		return dims
	}
	var arr []string
	if json.Unmarshal([]byte(jsonStr), &arr) == nil {
		return map[string][]string{"value": arr}
	}
	return nil
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

package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

// HelmTestPipelineInput defines the input for a helm test pipeline.
type HelmTestPipelineInput struct {
	Repo                  string            `json:"repo"`
	Ref                   string            `json:"ref"`
	HeadSHA               string            `json:"headSHA"`
	PRNumber              int               `json:"prNumber"`
	ChartPath             string            `json:"chartPath"`
	ReleaseName           string            `json:"releaseName"`
	Namespace             string            `json:"namespace"`
	Values                map[string]string `json:"values,omitempty"`
	Timeout               string            `json:"timeout"`
	ClusterPoolWorkflowID string            `json:"clusterPoolWorkflowID"`
}

// HelmTestPipelineResult defines the output of a helm test pipeline.
type HelmTestPipelineResult struct {
	Status        string  `json:"status"`
	ClusterName   string  `json:"clusterName"`
	InstallOutput string  `json:"installOutput"`
	TestOutput    string  `json:"testOutput"`
	Duration      float64 `json:"duration"`
}

// HelmTestPipeline orchestrates: lease cluster → clone → helm install → helm test → report → release.
func HelmTestPipeline(ctx workflow.Context, input HelmTestPipelineInput) (HelmTestPipelineResult, error) {
	startTime := workflow.Now(ctx)
	var acts *activities.Activities
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID

	if input.Namespace == "" {
		input.Namespace = "test"
	}
	if input.ReleaseName == "" {
		input.ReleaseName = "ci-test"
	}

	// 1. Set pending status
	reportCtx := withReportOptions(ctx)
	_ = workflow.ExecuteActivity(reportCtx, acts.SetCommitStatus, activities.StatusInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA,
		State: "pending", Description: "Helm test: leasing cluster...",
	}).Get(ctx, nil)

	// 2. Signal ClusterPool to lease a cluster
	requestID := wfID
	_ = workflow.SignalExternalWorkflow(ctx, input.ClusterPoolWorkflowID, "", "lease",
		LeaseSignal{RequestID: requestID, WorkflowID: wfID}).Get(ctx, nil)

	// 3. Wait for lease result (ClusterPool signals us back)
	var lease LeaseResult
	leaseTimeout := ParseTimeout("15m", 15*time.Minute)
	leaseCtx, leaseCancel := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(leaseCtx, leaseTimeout)
	leaseCh := workflow.GetSignalChannel(ctx, "lease-result")

	sel := workflow.NewSelector(ctx)
	leaseOK := false
	sel.AddReceive(leaseCh, func(ch workflow.ReceiveChannel, _ bool) {
		ch.Receive(ctx, &lease)
		leaseOK = true
		leaseCancel()
	})
	sel.AddFuture(timer, func(f workflow.Future) {})
	sel.Select(ctx)

	if !leaseOK {
		return HelmTestPipelineResult{Status: "failed"}, fmt.Errorf("timed out waiting for cluster lease")
	}

	// Ensure we release the cluster on exit
	defer func() {
		dCtx, _ := workflow.NewDisconnectedContext(ctx)
		_ = workflow.SignalExternalWorkflow(dCtx, input.ClusterPoolWorkflowID, "", "release",
			ReleaseSignal{ClusterName: lease.ClusterName}).Get(dCtx, nil)
	}()

	// 4. Clone repo
	cloneCtx := withCloneOptions(ctx)
	var cloneResult activities.CloneResult
	if err := workflow.ExecuteActivity(cloneCtx, acts.CloneRepo, activities.CloneInput{
		Repo: input.Repo, Ref: input.Ref, WorkflowID: wfID,
	}).Get(ctx, &cloneResult); err != nil {
		return HelmTestPipelineResult{Status: "failed", ClusterName: lease.ClusterName},
			fmt.Errorf("clone: %w", err)
	}

	// 5. Run helm test
	helmCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: ParseTimeout(input.Timeout, 10*time.Minute),
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})

	chartPath := cloneResult.Dir
	if input.ChartPath != "" {
		chartPath = cloneResult.Dir + "/" + input.ChartPath
	}

	var helmResult activities.HelmTestResult
	err := workflow.ExecuteActivity(helmCtx, acts.RunHelmTest, activities.HelmTestInput{
		Kubeconfig:  lease.Kubeconfig,
		ChartPath:   chartPath,
		ReleaseName: input.ReleaseName,
		Namespace:   input.Namespace,
		Values:      input.Values,
		Timeout:     input.Timeout,
	}).Get(ctx, &helmResult)

	status := "passed"
	if err != nil || !helmResult.Passed {
		status = "failed"
	}

	duration := workflow.Now(ctx).Sub(startTime).Seconds()

	// 6. Report results
	dCtx, _ := workflow.NewDisconnectedContext(ctx)
	dReportCtx := withReportOptions(dCtx)

	stepResult := activities.StepResult{
		Name:   "helm-test",
		Status: status,
		Output: activities.TruncateOutput(helmResult.InstallOutput+"\n---\n"+helmResult.TestOutput, 4000),
		Duration: duration,
	}

	_ = workflow.ExecuteActivity(dReportCtx, acts.ReportResults, activities.ReportInput{
		Repo: input.Repo, HeadSHA: input.HeadSHA, PRNumber: input.PRNumber,
		Steps: []activities.StepResult{stepResult}, WorkflowID: wfID,
	}).Get(dCtx, nil)

	// 7. Cleanup
	_ = workflow.ExecuteActivity(dReportCtx, acts.Cleanup, cloneResult.Dir).Get(dCtx, nil)

	return HelmTestPipelineResult{
		Status: status, ClusterName: lease.ClusterName,
		InstallOutput: helmResult.InstallOutput, TestOutput: helmResult.TestOutput,
		Duration: duration,
	}, nil
}

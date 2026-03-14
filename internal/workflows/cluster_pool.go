package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

// ClusterPoolState tracks the state of the cluster pool.
type ClusterPoolState struct {
	Warm   []string `json:"warm"`
	Leased []string `json:"leased"`
	MaxSize int     `json:"maxSize"`
}

// ClusterPool is a long-running workflow that manages a pool of EKS clusters.
func ClusterPool(ctx workflow.Context, poolName string, maxSize int) error {
	state := ClusterPoolState{MaxSize: maxSize}

	// Query handler for pool status
	_ = workflow.SetQueryHandler(ctx, "status", func() (ClusterPoolState, error) {
		return state, nil
	})

	// Signal channels
	leaseCh := workflow.GetSignalChannel(ctx, "lease-request")
	releaseCh := workflow.GetSignalChannel(ctx, "release-cluster")

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 20 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)
	var acts *activities.Activities

	for {
		// Use selector to handle signals and timers
		selector := workflow.NewSelector(ctx)

		// Handle lease requests
		selector.AddReceive(leaseCh, func(c workflow.ReceiveChannel, more bool) {
			var req activities.ClusterLeaseInput
			c.Receive(ctx, &req)

			if len(state.Warm) > 0 {
				// Lease from warm pool
				cluster := state.Warm[0]
				state.Warm = state.Warm[1:]
				state.Leased = append(state.Leased, cluster)
				// Signal back the result via a named signal
				workflow.GetSignalChannel(ctx, "lease-result-"+cluster).Send(ctx, cluster)
			} else if len(state.Warm)+len(state.Leased) < state.MaxSize {
				// Provision a new cluster
				var result activities.ClusterLeaseResult
				err := workflow.ExecuteActivity(actCtx, acts.ProvisionCluster, req).Get(ctx, &result)
				if err == nil {
					state.Leased = append(state.Leased, result.ClusterName)
				}
			}
		})

		// Handle cluster releases
		selector.AddReceive(releaseCh, func(c workflow.ReceiveChannel, more bool) {
			var rel activities.ClusterReleaseInput
			c.Receive(ctx, &rel)

			// Remove from leased
			for i, name := range state.Leased {
				if name == rel.ClusterName {
					state.Leased = append(state.Leased[:i], state.Leased[i+1:]...)
					break
				}
			}

			if rel.Destroy || len(state.Warm) >= state.MaxSize {
				_ = workflow.ExecuteActivity(actCtx, acts.DestroyCluster, rel.ClusterName).Get(ctx, nil)
			} else {
				state.Warm = append(state.Warm, rel.ClusterName)
			}
		})

		selector.Select(ctx)

		if ctx.Err() != nil {
			break
		}
	}

	return nil
}

// HelmTestPipeline runs a Helm test on a leased cluster.
func HelmTestPipeline(ctx workflow.Context, input HelmTestPipelineInput) (HelmTestPipelineResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var acts *activities.Activities

	// 1. Lease cluster
	var lease activities.ClusterLeaseResult
	err := workflow.ExecuteActivity(ctx, acts.LeaseCluster, activities.ClusterLeaseInput{
		Pool: input.ClusterPool,
		TTL:  input.ClusterTTL,
	}).Get(ctx, &lease)
	if err != nil {
		return HelmTestPipelineResult{Status: "failed"}, fmt.Errorf("lease cluster: %w", err)
	}

	// Ensure cluster is released on completion
	defer func() {
		disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
		releaseCtx := workflow.WithActivityOptions(disconnectedCtx, workflow.ActivityOptions{
			StartToCloseTimeout: 5 * time.Minute,
			RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
		})
		_ = workflow.ExecuteActivity(releaseCtx, acts.ReleaseCluster, activities.ClusterReleaseInput{
			ClusterName: lease.ClusterName,
		}).Get(disconnectedCtx, nil)
	}()

	// 2. Deploy Helm chart
	err = workflow.ExecuteActivity(ctx, acts.HelmDeploy, activities.HelmDeployInput{
		ClusterName: lease.ClusterName,
		Endpoint:    lease.Endpoint,
		CA:          lease.CA,
		Chart:       input.Chart,
		Values:      input.Values,
		ReleaseName: input.ReleaseName,
		Namespace:   "default",
		Dir:         input.Dir,
	}).Get(ctx, nil)
	if err != nil {
		return HelmTestPipelineResult{Status: "failed", ClusterName: lease.ClusterName}, fmt.Errorf("helm deploy: %w", err)
	}

	// 3. Run tests
	var testResult activities.HelmTestResult
	err = workflow.ExecuteActivity(ctx, acts.HelmTest, activities.HelmTestInput{
		ClusterName: lease.ClusterName,
		Endpoint:    lease.Endpoint,
		CA:          lease.CA,
		ReleaseName: input.ReleaseName,
		Namespace:   "default",
		TestCommand: input.TestCommand,
	}).Get(ctx, &testResult)

	status := "passed"
	if err != nil || testResult.ExitCode != 0 {
		status = "failed"
	}

	return HelmTestPipelineResult{
		Status:      status,
		Output:      testResult.Output,
		ExitCode:    testResult.ExitCode,
		ClusterName: lease.ClusterName,
	}, nil
}

// HelmTestPipelineInput defines input for the Helm test pipeline.
type HelmTestPipelineInput struct {
	Repo        string `json:"repo"`
	Ref         string `json:"ref"`
	Dir         string `json:"dir"`
	Chart       string `json:"chart"`
	Values      string `json:"values,omitempty"`
	ReleaseName string `json:"releaseName"`
	TestCommand string `json:"testCommand,omitempty"`
	ClusterPool string `json:"clusterPool"`
	ClusterTTL  string `json:"clusterTTL,omitempty"`
}

// HelmTestPipelineResult defines the output of the Helm test pipeline.
type HelmTestPipelineResult struct {
	Status      string `json:"status"`
	Output      string `json:"output"`
	ExitCode    int    `json:"exitCode"`
	ClusterName string `json:"clusterName"`
}

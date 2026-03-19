package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

// PodCleanup is a scheduled workflow that garbage-collects completed CI pods.
func PodCleanup(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	})
	var acts *activities.Activities
	var result activities.CleanupPodsResult
	return workflow.ExecuteActivity(ctx, acts.CleanupStalePods).Get(ctx, &result)
}

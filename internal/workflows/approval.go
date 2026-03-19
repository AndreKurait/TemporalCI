package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AndreKurait/TemporalCI/internal/activities"
)

const ApprovalSignalName = "approval"

// ApprovalSignal is sent to approve or reject a deployment.
type ApprovalSignal struct {
	Approved bool   `json:"approved"`
	Approver string `json:"approver"`
}

// ApprovalGateInput defines the input for the approval gate workflow.
type ApprovalGateInput struct {
	Repo        string  `json:"repo"`
	Ref         string  `json:"ref"`
	Environment string  `json:"environment"` // e.g. "production"
	SlackURL    string  `json:"slackURL"`
	Timeout     string  `json:"timeout"` // e.g. "1h"
	WorkflowID  string  `json:"workflowID"`
}

// ApprovalGateResult defines the output of the approval gate.
type ApprovalGateResult struct {
	Approved bool   `json:"approved"`
	Approver string `json:"approver"`
}

// ApprovalGate waits for a human approval signal before proceeding.
func ApprovalGate(ctx workflow.Context, input ApprovalGateInput) (ApprovalGateResult, error) {
	var acts *activities.Activities

	// Notify via Slack that approval is needed
	if input.SlackURL != "" {
		notifyCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Second,
			RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
		})
		_ = workflow.ExecuteActivity(notifyCtx, acts.NotifySlack, activities.NotifySlackInput{
			WebhookURL: input.SlackURL,
			Repo:       input.Repo,
			Status:     fmt.Sprintf("⏳ Awaiting approval for *%s* deploy", input.Environment),
			WorkflowID: input.WorkflowID,
		}).Get(ctx, nil)
	}

	// Wait for approval signal
	timeout := ParseTimeout(input.Timeout, 1*time.Hour)
	signalCh := workflow.GetSignalChannel(ctx, ApprovalSignalName)

	var signal ApprovalSignal
	timerCtx, cancel := workflow.WithCancel(ctx)
	timerFuture := workflow.NewTimer(timerCtx, timeout)

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(signalCh, func(ch workflow.ReceiveChannel, _ bool) {
		ch.Receive(ctx, &signal)
		cancel()
	})
	selector.AddFuture(timerFuture, func(f workflow.Future) {
		// Timeout — treat as rejected
		signal = ApprovalSignal{Approved: false, Approver: "timeout"}
	})
	selector.Select(ctx)

	return ApprovalGateResult{Approved: signal.Approved, Approver: signal.Approver}, nil
}

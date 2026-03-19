package workflows

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
)

// SyncSchedules creates/updates/deletes Temporal schedules based on pipeline config.
// Called when a repo's .temporalci.yaml changes (on push to default branch).
func SyncSchedules(ctx context.Context, tc client.Client, repo string, pipelines map[string]ScheduleSpec) error {
	sc := tc.ScheduleClient()

	for name, spec := range pipelines {
		scheduleID := fmt.Sprintf("schedule-%s-%s", repo, name)

		for _, cron := range spec.Crons {
			_, err := sc.Create(ctx, client.ScheduleOptions{
				ID: scheduleID,
				Spec: client.ScheduleSpec{
					CronExpressions: []string{cron},
				},
				Action: &client.ScheduleWorkflowAction{
					ID:        fmt.Sprintf("ci-%s-%s-scheduled", repo, name),
					Workflow:  "CIPipeline",
					TaskQueue: "temporalci-task-queue",
					Args: []interface{}{CIPipelineInput{
						Event:        "schedule",
						Repo:         repo,
						Ref:          spec.DefaultBranch,
						PipelineName: name,
					}},
				},
				// Default overlap policy is Skip
			})
			if err != nil {
				// Try update if already exists
				handle := sc.GetHandle(ctx, scheduleID)
				err = handle.Update(ctx, client.ScheduleUpdateOptions{
					DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
						input.Description.Schedule.Spec.CronExpressions = []string{cron}
						return &client.ScheduleUpdate{Schedule: &input.Description.Schedule}, nil
					},
				})
				if err != nil {
					slog.Warn("failed to sync schedule", "id", scheduleID, "error", err)
				}
			}
		}
	}
	return nil
}

// DeleteSchedule removes a schedule.
func DeleteSchedule(ctx context.Context, tc client.Client, repo, pipeline string) error {
	scheduleID := fmt.Sprintf("schedule-%s-%s", repo, pipeline)
	handle := tc.ScheduleClient().GetHandle(ctx, scheduleID)
	return handle.Delete(ctx)
}

// ScheduleSpec defines what to schedule for a pipeline.
type ScheduleSpec struct {
	Crons         []string
	DefaultBranch string
}

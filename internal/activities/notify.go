package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// NotifySlackInput defines the input for the NotifySlack activity.
type NotifySlackInput struct {
	WebhookURL string `json:"webhookURL"`
	Repo       string `json:"repo"`
	Ref        string `json:"ref"`
	Status     string `json:"status"` // passed, failed, cancelled
	StepCount  int    `json:"stepCount"`
	Duration   float64 `json:"duration"`
	WorkflowID string `json:"workflowID"`
}

// NotifySlack sends a pipeline completion notification to Slack.
func (a *Activities) NotifySlack(ctx context.Context, input NotifySlackInput) error {
	if input.WebhookURL == "" {
		return nil
	}

	icon := "✅"
	if input.Status == "failed" || input.Status == "cancelled" {
		icon = "❌"
	}

	text := fmt.Sprintf("%s *%s* — `%s` %s (%d steps, %.1fs)",
		icon, input.Repo, trimRef(input.Ref), input.Status, input.StepCount, input.Duration)

	if a.TemporalWebURL != "" && input.WorkflowID != "" {
		text += fmt.Sprintf("\n<%s|View workflow>", WorkflowURL(a.TemporalWebURL, input.WorkflowID))
	}

	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, input.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}

package activities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v70/github"
)

// CheckRunInput defines the input for creating GitHub Check Runs.
type CheckRunInput struct {
	Repo         string       `json:"repo"`
	HeadSHA      string       `json:"headSHA"`
	Steps        []StepResult `json:"steps"`
	WorkflowID   string       `json:"workflowID"`
	PipelineName string       `json:"pipelineName,omitempty"`
}

// CreateCheckRuns creates GitHub Check Runs with per-step annotations.
func (a *Activities) CreateCheckRuns(ctx context.Context, input CheckRunInput) error {
	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return err
	}

	owner, repo, err := splitRepo(input.Repo)
	if err != nil {
		return err
	}

	for _, step := range input.Steps {
		conclusion := "success"
		switch step.Status {
		case "failed":
			conclusion = "failure"
		case "cancelled":
			conclusion = "cancelled"
		case "skipped":
			conclusion = "skipped"
		}

		summary := fmt.Sprintf("**%s** — %s", step.Name, step.Status)
		if step.Duration > 0.1 {
			summary += fmt.Sprintf(" (%.1fs)", step.Duration)
		}

		var text string
		if step.Output != "" {
			// Inline last ~50 lines, up to 65KB limit
			text = truncateForCheckRun(step.Output, 60000)
		}

		status := "completed"
		opts := github.CreateCheckRunOptions{
			Name:        fmt.Sprintf("TemporalCI / %s", step.Name),
			HeadSHA:     input.HeadSHA,
			Status:      &status,
			Conclusion:  &conclusion,
			CompletedAt: &github.Timestamp{Time: time.Now()},
			Output: &github.CheckRunOutput{
				Title:   github.String(fmt.Sprintf("%s: %s", step.Name, step.Status)),
				Summary: &summary,
			},
		}

		if text != "" {
			opts.Output.Text = &text
		}

		if step.LogURL != "" {
			opts.DetailsURL = &step.LogURL
		} else if a.TemporalWebURL != "" && input.WorkflowID != "" {
			u := WorkflowURL(a.TemporalWebURL, input.WorkflowID)
			opts.DetailsURL = &u
		}

		if _, _, err := gh.Checks.CreateCheckRun(ctx, owner, repo, opts); err != nil {
			a.logger(ctx).Warn("failed to create check run", "step", step.Name, "error", err)
		}
	}

	return nil
}

// githubClient returns a GitHub client, preferring App auth over PAT.
func (a *Activities) githubClient(ctx context.Context, repoFullName string) (*github.Client, error) {
	if a.GitHubApp != nil {
		owner, repo, err := splitRepo(repoFullName)
		if err != nil {
			return nil, err
		}
		installID, err := a.GitHubApp.FindInstallation(ctx, owner, repo)
		if err != nil {
			// Fall back to PAT if App can't find installation
			if a.GitHubToken != "" {
				return github.NewClient(nil).WithAuthToken(a.GitHubToken), nil
			}
			return nil, fmt.Errorf("no GitHub auth available: %w", err)
		}
		return a.GitHubApp.InstallationClient(ctx, installID)
	}
	if a.GitHubToken != "" {
		return github.NewClient(nil).WithAuthToken(a.GitHubToken), nil
	}
	return nil, nil
}

func truncateForCheckRun(s string, maxLen int) string {
	if len(s) <= maxLen {
		return "```\n" + s + "\n```"
	}
	lines := strings.Split(s, "\n")
	// Take last ~50 lines
	start := len(lines) - 50
	if start < 0 {
		start = 0
	}
	truncated := strings.Join(lines[start:], "\n")
	if len(truncated) > maxLen {
		truncated = truncated[len(truncated)-maxLen:]
	}
	return "```\n... (truncated)\n" + truncated + "\n```"
}

package activities

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v67/github"
	"go.temporal.io/sdk/activity"
	"k8s.io/client-go/kubernetes"

	"github.com/AndreKurait/TemporalCI/internal/config"
	"github.com/AndreKurait/TemporalCI/internal/k8s"
)

// Activities holds shared dependencies for all activity methods.
type Activities struct {
	K8sClient      kubernetes.Interface
	GitHubToken    string
	TemporalWebURL string
	LogBucket      string
	AWSRegion      string
}

// CloneRepo clones a repository at the given ref.
func (a *Activities) CloneRepo(ctx context.Context, input CloneInput) (CloneResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Cloning repo", "repo", input.Repo, "ref", input.Ref)

	dir := fmt.Sprintf("/tmp/ci/%s", input.WorkflowID)

	// Clean up any previous clone
	_ = os.RemoveAll(dir)

	cloneURL := fmt.Sprintf("https://github.com/%s.git", input.Repo)
	branch := strings.TrimPrefix(input.Ref, "refs/heads/")
	branch = strings.TrimPrefix(branch, "refs/tags/")
	if err := runCmd(ctx, "", "git", "clone", "--depth=1", "--branch", branch, cloneURL, dir); err != nil {
		return CloneResult{}, fmt.Errorf("git clone: %w", err)
	}

	// Load pipeline config from cloned repo
	var steps []StepConfig
	if pCfg, err := config.LoadPipelineConfig(dir); err == nil {
		for _, s := range pCfg.Steps {
			steps = append(steps, StepConfig{Name: s.Name, Image: s.Image, Command: s.Command, Timeout: s.Timeout, DependsOn: s.DependsOn})
		}
	}

	return CloneResult{Dir: dir, Steps: steps}, nil
}

// RunStep executes a single CI step in a K8s pod, or locally if K8sClient is nil.
func (a *Activities) RunStep(ctx context.Context, input RunStepInput) (RunStepResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Running step", "name", input.Name, "image", input.Image)

	if a.K8sClient != nil {
		info := activity.GetInfo(ctx)
		podName := fmt.Sprintf("ci-%s-%s", input.Name, info.ActivityID)
		result, err := k8s.RunPod(ctx, a.K8sClient, k8s.PodSpec{
			Name:       podName,
			Namespace:  "temporalci",
			Image:      input.Image,
			Command:    []string{"sh", "-c", input.Command},
			WorkingDir: input.Dir,
			Tolerations: []string{"ci-jobs"},
		})
		if err != nil {
			return RunStepResult{}, fmt.Errorf("k8s pod: %w", err)
		}
		return RunStepResult{
			ExitCode: result.ExitCode,
			Output:   truncateOutput(result.Logs, 4000),
		}, nil
	}

	// Local mode fallback: run command directly via shell
	cmd := exec.CommandContext(ctx, "sh", "-c", input.Command)
	cmd.Dir = input.Dir
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return RunStepResult{}, fmt.Errorf("exec: %w", err)
		}
	}

	return RunStepResult{
		ExitCode: exitCode,
		Output:   truncateOutput(string(out), 4000),
	}, nil
}

// Cleanup removes the clone directory after a workflow completes.
func (a *Activities) Cleanup(ctx context.Context, dir string) error {
	return os.RemoveAll(dir)
}

// truncateOutput keeps the last maxLen bytes, prepending a truncation notice.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "... (truncated)\n" + s[len(s)-maxLen:]
}

// SetCommitStatus sets a commit status on GitHub (pending, success, failure).
func (a *Activities) SetCommitStatus(ctx context.Context, input StatusInput) error {
	if a.GitHubToken == "" {
		return nil
	}
	gh := github.NewClient(nil).WithAuthToken(a.GitHubToken)
	parts := strings.SplitN(input.Repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo: %s", input.Repo)
	}
	ciContext := "TemporalCI"
	_, _, err := gh.Repositories.CreateStatus(ctx, parts[0], parts[1], input.HeadSHA, &github.RepoStatus{
		State: &input.State, Description: &input.Description, Context: &ciContext,
	})
	return err
}

// ReportResults reports CI results back to GitHub via commit status and PR comments.
func (a *Activities) ReportResults(ctx context.Context, input ReportInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Reporting results", "repo", input.Repo, "sha", input.HeadSHA, "steps", len(input.Steps))

	if a.GitHubToken == "" {
		logger.Warn("No GitHub token configured, skipping report")
		return nil
	}

	gh := github.NewClient(nil).WithAuthToken(a.GitHubToken)
	parts := strings.SplitN(input.Repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo format: %s", input.Repo)
	}
	owner, repo := parts[0], parts[1]

	// Determine overall state
	state := "success"
	for _, s := range input.Steps {
		if s.Status == "failed" || s.Status == "cancelled" {
			state = "failure"
			break
		}
	}

	// Build description
	var summary strings.Builder
	for _, s := range input.Steps {
		icon := "✅"
		if s.Status == "failed" {
			icon = "❌"
		} else if s.Status == "skipped" {
			icon = "⏭️"
		} else if s.Status == "cancelled" {
			icon = "🚫"
		}
		if s.Duration > 0.1 {
			fmt.Fprintf(&summary, "%s **%s** (exit %d, %.1fs)\n", icon, s.Name, s.ExitCode, s.Duration)
		} else {
			fmt.Fprintf(&summary, "%s **%s** (exit %d)\n", icon, s.Name, s.ExitCode)
		}
	}

	// Create commit status (works with PATs, unlike Check Runs)
	description := fmt.Sprintf("CI %s (%d steps)", state, len(input.Steps))
	if len(description) > 140 {
		description = description[:140]
	}
	ciContext := "TemporalCI"
	status := &github.RepoStatus{
		State: &state, Description: &description, Context: &ciContext,
	}
	if a.TemporalWebURL != "" && input.WorkflowID != "" {
		targetURL := fmt.Sprintf("%s/namespaces/default/workflows/%s", a.TemporalWebURL, input.WorkflowID)
		status.TargetURL = &targetURL
	}
	_, _, err := gh.Repositories.CreateStatus(ctx, owner, repo, input.HeadSHA, status)
	if err != nil {
		return fmt.Errorf("create commit status: %w", err)
	}
	logger.Info("Created commit status", "state", state)

	// Post PR comment if this is a pull request
	if input.PRNumber > 0 {
		body := fmt.Sprintf("## TemporalCI Results\n\n%s\nConclusion: **%s**", summary.String(), state)
		_, _, err := gh.Issues.CreateComment(ctx, owner, repo, input.PRNumber, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("create PR comment: %w", err)
		}
		logger.Info("Posted PR comment", "pr", input.PRNumber)
	}

	return nil
}

// runCmd executes a command with context, optionally in a directory.
func runCmd(ctx context.Context, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}
	return nil
}

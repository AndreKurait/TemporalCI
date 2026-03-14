package activities

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/go-github/v67/github"
	"go.temporal.io/sdk/activity"
	"k8s.io/client-go/kubernetes"

	"github.com/AndreKurait/TemporalCI/internal/k8s"
)

// Activities holds shared dependencies for all activity methods.
type Activities struct {
	K8sClient   kubernetes.Interface
	GitHubToken string
	LogBucket   string
	AWSRegion   string
}

// CloneRepo clones a repository at the given ref.
func (a *Activities) CloneRepo(ctx context.Context, input CloneInput) (CloneResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Cloning repo", "repo", input.Repo, "ref", input.Ref)

	dir := fmt.Sprintf("/tmp/ci/%s/%s", input.Repo, input.Ref)

	cloneURL := fmt.Sprintf("https://github.com/%s.git", input.Repo)
	if err := runCmd(ctx, "", "git", "clone", "--depth=1", cloneURL, dir); err != nil {
		return CloneResult{}, fmt.Errorf("git clone: %w", err)
	}

	if err := runCmd(ctx, dir, "git", "checkout", input.Ref); err != nil {
		return CloneResult{}, fmt.Errorf("git checkout: %w", err)
	}

	return CloneResult{Dir: dir}, nil
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
			Output:   result.Logs,
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
		Output:   string(out),
	}, nil
}

// ReportResults reports CI results back to GitHub as a Check Run.
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

	// Determine conclusion from step results
	conclusion := "success"
	for _, s := range input.Steps {
		if s.Status == "failed" {
			conclusion = "failure"
			break
		}
	}

	// Build summary
	var summary strings.Builder
	for _, s := range input.Steps {
		icon := "✅"
		if s.Status == "failed" {
			icon = "❌"
		}
		fmt.Fprintf(&summary, "%s **%s** (exit %d)\n", icon, s.Name, s.ExitCode)
	}

	status := "completed"
	checkRun, _, err := gh.Checks.CreateCheckRun(ctx, owner, repo, github.CreateCheckRunOptions{
		Name:       "TemporalCI",
		HeadSHA:    input.HeadSHA,
		Status:     &status,
		Conclusion: &conclusion,
		Output: &github.CheckRunOutput{
			Title:   github.String(fmt.Sprintf("CI %s", conclusion)),
			Summary: github.String(summary.String()),
		},
	})
	if err != nil {
		return fmt.Errorf("create check run: %w", err)
	}
	logger.Info("Created check run", "id", checkRun.GetID())

	// Post PR comment if this is a pull request
	if input.PRNumber > 0 {
		body := fmt.Sprintf("## TemporalCI Results\n\n%s\n\nConclusion: **%s**", summary.String(), conclusion)
		_, _, err := gh.Issues.CreateComment(ctx, owner, repo, input.PRNumber, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("create PR comment: %w", err)
		}
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

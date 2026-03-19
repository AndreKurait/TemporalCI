package activities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-github/v67/github"
	"go.temporal.io/sdk/activity"
	"k8s.io/client-go/kubernetes"

	"github.com/AndreKurait/TemporalCI/internal/config"
	"github.com/AndreKurait/TemporalCI/internal/ghapp"
	"github.com/AndreKurait/TemporalCI/internal/k8s"
	"github.com/AndreKurait/TemporalCI/internal/metrics"
)

// Activities holds shared dependencies for all activity methods.
type Activities struct {
	K8sClient      kubernetes.Interface
	GitHubToken    string
	GitHubApp      *ghapp.Client // GitHub App auth (preferred over PAT)
	TemporalWebURL string
	Namespace      string
	S3Client       S3Uploader
	S3Presigner    S3Presigner
	LogBucket      string
	CINodePool     bool
	SecretsClient  SecretsClient
	SecretsPrefix  string
}

func (a *Activities) namespace() string {
	if a.Namespace != "" {
		return a.Namespace
	}
	if ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return strings.TrimSpace(string(ns))
	}
	return "default"
}

func (a *Activities) logger(ctx context.Context) *slog.Logger {
	info := activity.GetInfo(ctx)
	return slog.Default().With(
		"workflowID", info.WorkflowExecution.ID,
		"activityType", info.ActivityType.Name,
	)
}

// CloneRepo clones a repository at the given ref.
func (a *Activities) CloneRepo(ctx context.Context, input CloneInput) (CloneResult, error) {
	log := a.logger(ctx).With("repo", input.Repo, "ref", input.Ref)
	log.Info("cloning repo")

	dir := fmt.Sprintf("/tmp/ci/%s", input.WorkflowID)
	_ = os.RemoveAll(dir)

	cloneURL := fmt.Sprintf("https://github.com/%s.git", input.Repo)
	branch := trimRef(input.Ref)
	if err := runCmd(ctx, "", "git", "clone", "--depth=1", "--branch", branch, cloneURL, dir); err != nil {
		return CloneResult{}, fmt.Errorf("git clone: %w", err)
	}

	var steps []StepConfig
	if pCfg, err := config.LoadPipelineConfig(dir); err == nil {
		for _, s := range pCfg.Steps {
			sc := StepConfig{
				Name: s.Name, Image: s.Image, Command: s.Command,
				Timeout: s.Timeout, DependsOn: s.DependsOn,
				Secrets: s.Secrets,
			}
			if s.Resources != nil {
				sc.Resources = &ResourceConfig{CPU: s.Resources.CPU, Memory: s.Resources.Memory}
			}
			steps = append(steps, sc)
		}
	}

	return CloneResult{Dir: dir, Steps: steps}, nil
}

// RunStep executes a single CI step as a K8s pod or locally.
func (a *Activities) RunStep(ctx context.Context, input RunStepInput) (RunStepResult, error) {
	log := a.logger(ctx).With("step", input.Name, "image", input.Image)
	log.Info("running step")

	if a.K8sClient != nil {
		return a.runStepK8s(ctx, input)
	}
	return a.runStepLocal(ctx, input)
}

func (a *Activities) runStepK8s(ctx context.Context, input RunStepInput) (RunStepResult, error) {
	metrics.PodsActive.Inc()
	defer metrics.PodsActive.Dec()

	info := activity.GetInfo(ctx)
	h := sha256.Sum256([]byte(info.WorkflowExecution.ID + info.ActivityID))
	podName := fmt.Sprintf("ci-%s-%s", input.Name, hex.EncodeToString(h[:6]))

	spec := k8s.PodSpec{
		Name:      podName,
		Namespace: a.namespace(),
		Image:     input.Image,
		Command:   []string{"sh", "-c", input.Command},
	}
	if input.Repo != "" {
		spec.CloneURL = fmt.Sprintf("https://github.com/%s.git", input.Repo)
		spec.CloneRef = trimRef(input.Ref)
	} else {
		spec.WorkingDir = input.Dir
	}
	if input.Resources != nil {
		spec.CPU = input.Resources.CPU
		spec.Memory = input.Resources.Memory
	}
	if a.CINodePool {
		spec.Tolerations = []string{"ci-jobs"}
		spec.NodeSelector = map[string]string{"workload": "ci-jobs"}
	}

	// Inject secrets as env vars
	if len(input.Secrets) > 0 {
		spec.Env = input.Secrets
	}

	result, err := k8s.RunPod(ctx, a.K8sClient, spec)
	if err != nil {
		return RunStepResult{}, fmt.Errorf("k8s pod: %w", err)
	}

	// Upload full logs to S3 if configured
	var logURL string
	if a.S3Client != nil && a.LogBucket != "" {
		uploadResult, uploadErr := a.UploadLog(ctx, UploadLogInput{
			WorkflowID: info.WorkflowExecution.ID,
			StepName:   input.Name,
			Logs:       result.Logs,
		})
		if uploadErr != nil {
			a.logger(ctx).Warn("failed to upload log", "error", uploadErr)
		} else {
			logURL = uploadResult.LogURL
		}
	}

	return RunStepResult{
		ExitCode: result.ExitCode,
		Output:   TruncateOutput(result.Logs, 4000),
		LogURL:   logURL,
	}, nil
}

func (a *Activities) runStepLocal(ctx context.Context, input RunStepInput) (RunStepResult, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", input.Command)
	cmd.Dir = input.Dir
	// Inject secrets into local env
	for k, v := range input.Secrets {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("%s=%s", k, v))
	}
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return RunStepResult{}, fmt.Errorf("exec: %w", err)
		}
	}
	return RunStepResult{ExitCode: exitCode, Output: TruncateOutput(string(out), 4000)}, nil
}

// Cleanup removes the clone directory.
func (a *Activities) Cleanup(ctx context.Context, dir string) error {
	return os.RemoveAll(dir)
}

// SetCommitStatus sets a commit status on GitHub.
func (a *Activities) SetCommitStatus(ctx context.Context, input StatusInput) error {
	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return err
	}
	owner, repo, err := splitRepo(input.Repo)
	if err != nil {
		return err
	}
	ciContext := "TemporalCI"
	_, _, err = gh.Repositories.CreateStatus(ctx, owner, repo, input.HeadSHA, &github.RepoStatus{
		State: &input.State, Description: &input.Description, Context: &ciContext,
	})
	return err
}

// ReportResults reports CI results to GitHub via Check Runs, commit status, and PR comment.
func (a *Activities) ReportResults(ctx context.Context, input ReportInput) error {
	log := a.logger(ctx).With("repo", input.Repo, "sha", input.HeadSHA, "steps", len(input.Steps))
	log.Info("reporting results")

	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return err
	}

	owner, repo, err := splitRepo(input.Repo)
	if err != nil {
		return err
	}

	state := "success"
	for _, s := range input.Steps {
		if s.Status == "failed" || s.Status == "cancelled" {
			state = "failure"
			break
		}
	}

	// Record metrics
	for _, s := range input.Steps {
		metrics.StepStatus.WithLabelValues(s.Name, s.Status).Inc()
	}

	// Build comment body
	var summary, details strings.Builder
	var totalDuration float64
	passed, failed := 0, 0

	for _, s := range input.Steps {
		icon := "✅"
		switch s.Status {
		case "failed":
			icon = "❌"
			failed++
		case "skipped":
			icon = "⏭️"
		case "cancelled":
			icon = "🚫"
			failed++
		default:
			passed++
		}
		totalDuration += s.Duration

		stepLine := fmt.Sprintf("%s **%s**", icon, s.Name)
		if s.Duration > 0.1 {
			stepLine += fmt.Sprintf(" (%.1fs)", s.Duration)
		}
		if s.LogURL != "" {
			stepLine += fmt.Sprintf(" — [Full log](%s)", s.LogURL)
		}
		fmt.Fprintln(&summary, stepLine)

		if s.Output != "" {
			fmt.Fprintf(&details, "\n<details>\n<summary>📋 <b>%s</b> — exit %d</summary>\n\n```\n%s```\n</details>\n", s.Name, s.ExitCode, s.Output)
		}
	}

	// Commit status
	description := fmt.Sprintf("CI %s (%d steps)", state, len(input.Steps))
	ciContext := "TemporalCI"
	status := &github.RepoStatus{State: &state, Description: &description, Context: &ciContext}
	if a.TemporalWebURL != "" && input.WorkflowID != "" {
		targetURL := WorkflowURL(a.TemporalWebURL, input.WorkflowID)
		status.TargetURL = &targetURL
	}
	if _, _, err := gh.Repositories.CreateStatus(ctx, owner, repo, input.HeadSHA, status); err != nil {
		return fmt.Errorf("create commit status: %w", err)
	}

	// PR comment
	if input.PRNumber > 0 {
		var body strings.Builder
		fmt.Fprintf(&body, "## TemporalCI Results\n\n%s\n", summary.String())
		if totalDuration > 0.1 {
			fmt.Fprintf(&body, "**%d passed**, **%d failed** in **%.1fs**\n", passed, failed, totalDuration)
		}
		if a.TemporalWebURL != "" && input.WorkflowID != "" {
			fmt.Fprintf(&body, "\n🔗 [View workflow run](%s)\n", WorkflowURL(a.TemporalWebURL, input.WorkflowID))
		}
		if details.Len() > 0 {
			fmt.Fprintf(&body, "\n### Step Logs\n%s", details.String())
		}
		comment := body.String()

		if err := upsertPRComment(ctx, gh, owner, repo, input.PRNumber, comment); err != nil {
			return fmt.Errorf("PR comment: %w", err)
		}
		log.Info("updated PR comment", "pr", input.PRNumber)
	}

	return nil
}

// upsertPRComment finds an existing TemporalCI comment and updates it, or creates a new one.
func upsertPRComment(ctx context.Context, gh *github.Client, owner, repo string, prNumber int, body string) error {
	comments, _, err := gh.Issues.ListComments(ctx, owner, repo, prNumber, &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return err
	}

	for _, c := range comments {
		if strings.HasPrefix(c.GetBody(), "## TemporalCI Results") {
			_, _, err := gh.Issues.EditComment(ctx, owner, repo, c.GetID(), &github.IssueComment{Body: &body})
			return err
		}
	}

	_, _, err = gh.Issues.CreateComment(ctx, owner, repo, prNumber, &github.IssueComment{Body: &body})
	return err
}

// --- Helpers ---

// TruncateOutput keeps the last maxLen bytes with a truncation notice.
func TruncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "... (truncated)\n" + s[len(s)-maxLen:]
}

// WorkflowURL builds a URL to the Temporal Web UI for a workflow.
func WorkflowURL(baseURL, workflowID string) string {
	return fmt.Sprintf("%s/namespaces/default/workflows/%s", baseURL, url.PathEscape(workflowID))
}

func trimRef(ref string) string {
	ref = strings.TrimPrefix(ref, "refs/heads/")
	ref = strings.TrimPrefix(ref, "refs/tags/")
	return ref
}

func splitRepo(repo string) (string, string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo: %s", repo)
	}
	return parts[0], parts[1], nil
}

func runCmd(ctx context.Context, dir, name string, args ...string) error {
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

package activities

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"k8s.io/client-go/kubernetes"
)

// Activities holds shared dependencies for all activity methods.
type Activities struct {
	K8sClient    kubernetes.Interface
	GitHubToken  string
	LogBucket    string
	AWSRegion    string
}

// CloneRepo clones a repository at the given ref.
func (a *Activities) CloneRepo(ctx context.Context, input CloneInput) (CloneResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Cloning repo", "repo", input.Repo, "ref", input.Ref)

	dir := fmt.Sprintf("/tmp/ci/%s/%s", input.Repo, input.Ref)
	// TODO: implement git clone via K8s job pod
	return CloneResult{Dir: dir}, nil
}

// RunStep executes a single CI step in a K8s pod.
func (a *Activities) RunStep(ctx context.Context, input RunStepInput) (RunStepResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Running step", "name", input.Name, "image", input.Image)

	// TODO: create K8s pod, stream logs, collect results
	return RunStepResult{ExitCode: 0, Output: fmt.Sprintf("step %q completed", input.Name)}, nil
}

// ReportResults reports CI results back to GitHub.
func (a *Activities) ReportResults(ctx context.Context, input ReportInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Reporting results", "repo", input.Repo, "sha", input.HeadSHA, "steps", len(input.Steps))

	// TODO: create GitHub Check Run via go-github client
	return nil
}

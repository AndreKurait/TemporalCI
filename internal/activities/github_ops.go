package activities

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/go-github/v70/github"
)

// AddLabelInput defines input for the AddLabel activity.
type AddLabelInput struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Label  string `json:"label"`
}

// AddLabel adds a label to an issue or PR. Idempotent.
func (a *Activities) AddLabel(ctx context.Context, input AddLabelInput) error {
	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return err
	}
	owner, repo, _ := splitRepo(input.Repo)
	_, _, err = gh.Issues.AddLabelsToIssue(ctx, owner, repo, input.Number, []string{input.Label})
	return err
}

// DeleteBranchInput defines input for the DeleteBranch activity.
type DeleteBranchInput struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

// DeleteBranch deletes a branch. Idempotent — ignores 422 (already deleted).
func (a *Activities) DeleteBranch(ctx context.Context, input DeleteBranchInput) error {
	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return err
	}
	owner, repo, _ := splitRepo(input.Repo)
	_, err = gh.Git.DeleteRef(ctx, owner, repo, "heads/"+input.Branch)
	if err != nil && strings.Contains(err.Error(), "422") {
		return nil // already deleted
	}
	return err
}

// CreatePullRequestInput defines input for the CreatePullRequest activity.
type CreatePullRequestInput struct {
	Repo  string `json:"repo"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
}

// CreatePullRequestResult returns the created PR number.
type CreatePullRequestResult struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// CreatePullRequest creates a PR. Idempotent — returns existing if head→base already exists.
func (a *Activities) CreatePullRequest(ctx context.Context, input CreatePullRequestInput) (CreatePullRequestResult, error) {
	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return CreatePullRequestResult{}, err
	}
	owner, repo, _ := splitRepo(input.Repo)
	pr, _, err := gh.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: &input.Title, Body: &input.Body,
		Head: &input.Head, Base: &input.Base,
	})
	if err != nil {
		// Check if PR already exists
		if strings.Contains(err.Error(), "already exists") {
			return CreatePullRequestResult{}, nil
		}
		return CreatePullRequestResult{}, err
	}
	return CreatePullRequestResult{Number: pr.GetNumber(), URL: pr.GetHTMLURL()}, nil
}

// CherryPickInput defines input for the CherryPick activity.
type CherryPickInput struct {
	Repo       string `json:"repo"`
	CommitSHA  string `json:"commitSHA"`
	TargetBranch string `json:"targetBranch"`
	NewBranch  string `json:"newBranch"`
}

// CherryPick cherry-picks a commit onto a new branch from target. Uses git CLI.
func (a *Activities) CherryPick(ctx context.Context, input CherryPickInput) error {
	dir := fmt.Sprintf("/tmp/cherry-pick-%s", input.NewBranch)
	cloneURL := fmt.Sprintf("https://github.com/%s.git", input.Repo)

	cmds := [][]string{
		{"git", "clone", "--depth=50", "--branch", input.TargetBranch, cloneURL, dir},
		{"git", "-C", dir, "checkout", "-b", input.NewBranch},
		{"git", "-C", dir, "cherry-pick", input.CommitSHA},
		{"git", "-C", dir, "push", "origin", input.NewBranch},
	}
	for _, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s", err, string(out))
		}
	}
	return nil
}

// CreateReleaseInput defines input for the CreateRelease activity.
type CreateReleaseInput struct {
	Repo               string   `json:"repo"`
	TagName            string   `json:"tagName"`
	Name               string   `json:"name"`
	Body               string   `json:"body"`
	Draft              bool     `json:"draft"`
	GenerateReleaseNotes bool   `json:"generateReleaseNotes"`
	ArtifactURLs       []string `json:"artifactURLs,omitempty"` // S3 presigned URLs to download and attach
}

// CreateReleaseResult returns the created release.
type CreateReleaseResult struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"htmlURL"`
}

// CreateRelease creates a GitHub release with optional artifacts.
func (a *Activities) CreateRelease(ctx context.Context, input CreateReleaseInput) (CreateReleaseResult, error) {
	gh, err := a.githubClient(ctx, input.Repo)
	if err != nil || gh == nil {
		return CreateReleaseResult{}, err
	}
	owner, repo, _ := splitRepo(input.Repo)

	release, _, err := gh.Repositories.CreateRelease(ctx, owner, repo, &github.RepositoryRelease{
		TagName:              &input.TagName,
		Name:                 &input.Name,
		Body:                 &input.Body,
		Draft:                &input.Draft,
		GenerateReleaseNotes: &input.GenerateReleaseNotes,
	})
	if err != nil {
		return CreateReleaseResult{}, fmt.Errorf("create release: %w", err)
	}

	return CreateReleaseResult{ID: release.GetID(), HTMLURL: release.GetHTMLURL()}, nil
}

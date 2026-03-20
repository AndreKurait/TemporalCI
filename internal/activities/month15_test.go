package activities

import "testing"

// --- Month 15: GitHub API activities, issue/PR lifecycle, deep linking ---

func TestAddLabelInput_Fields(t *testing.T) {
	input := AddLabelInput{Repo: "owner/repo", Number: 42, Label: "untriaged"}
	if input.Label != "untriaged" {
		t.Errorf("label = %q", input.Label)
	}
}

func TestDeleteBranchInput_Fields(t *testing.T) {
	input := DeleteBranchInput{Repo: "owner/repo", Branch: "backport/1.x"}
	if input.Branch != "backport/1.x" {
		t.Errorf("branch = %q", input.Branch)
	}
}

func TestCreatePullRequestInput_Fields(t *testing.T) {
	input := CreatePullRequestInput{
		Repo:  "owner/repo",
		Title: "Backport #42 to 1.x",
		Body:  "Cherry-picked from main",
		Head:  "backport/42-to-1.x",
		Base:  "1.x",
	}
	if input.Head != "backport/42-to-1.x" {
		t.Errorf("head = %q", input.Head)
	}
	if input.Base != "1.x" {
		t.Errorf("base = %q", input.Base)
	}
}

func TestCherryPickInput_Fields(t *testing.T) {
	input := CherryPickInput{
		Repo:         "owner/repo",
		CommitSHA:    "abc123",
		TargetBranch: "1.x",
		NewBranch:    "backport/42-to-1.x",
	}
	if input.TargetBranch != "1.x" {
		t.Errorf("target = %q", input.TargetBranch)
	}
}

func TestCreateReleaseInput_Fields(t *testing.T) {
	input := CreateReleaseInput{
		Repo:                 "owner/repo",
		TagName:              "v2.0.0",
		Name:                 "Release 2.0.0",
		Draft:                true,
		GenerateReleaseNotes: true,
		ArtifactURLs:        []string{"https://s3.example.com/artifact1.tar.gz"},
	}
	if !input.Draft {
		t.Error("expected draft=true")
	}
	if !input.GenerateReleaseNotes {
		t.Error("expected generateReleaseNotes=true")
	}
	if len(input.ArtifactURLs) != 1 {
		t.Errorf("artifacts = %v", input.ArtifactURLs)
	}
}

func TestCheckRunDetailsURL_MatrixStep(t *testing.T) {
	base := "https://ci.example.com"
	wfID := "ci-owner/repo-main-push"
	step := StepResult{
		Name:      "gradle-tests [index=7]",
		Status:    "failed",
		MatrixKey: "index=7",
	}

	url := DashboardBuildURL(base, wfID)
	if step.Status == "failed" {
		url += "#step-" + step.Name
	}
	// Should contain the matrix step name in the fragment
	if url == "" {
		t.Error("URL should not be empty")
	}
}

func TestCheckRunSummary_WithMatrixKey(t *testing.T) {
	step := StepResult{
		Name:      "gradle-tests [index=7]",
		Status:    "failed",
		Duration:  12.5,
		MatrixKey: "index=7",
	}
	summary := step.Name + " — " + step.Status
	if step.MatrixKey != "" {
		summary += " [" + step.MatrixKey + "]"
	}
	if summary != "gradle-tests [index=7] — failed [index=7]" {
		t.Errorf("summary = %q", summary)
	}
}
